package main

import (
	"sort"
)

// PhotoGroup represents a group of similar photos
type PhotoGroup struct {
	GroupID    int     `json:"group_id"`
	PhotoIDs   []int64 `json:"photo_ids"`
	AvgSimilarity float64 `json:"avg_similarity"` // Average pairwise similarity within group
}

// ClusteringResult represents the result of photo clustering
type ClusteringResult struct {
	Groups    []PhotoGroup `json:"groups"`
	Ungrouped []int64      `json:"ungrouped"` // Photos that don't belong to any group
}

// DBSCAN implements density-based spatial clustering
// eps: maximum distance (1 - similarity) between two points to be considered neighbors
// minPts: minimum number of points to form a dense region (cluster)
type DBSCAN struct {
	Eps    float64 // e.g., 0.25 means similarity >= 0.75
	MinPts int     // e.g., 2 means at least 2 similar photos to form a group
}

// NewDBSCAN creates a new DBSCAN clusterer with default parameters
func NewDBSCAN() *DBSCAN {
	return &DBSCAN{
		Eps:    0.25, // 75% similarity threshold
		MinPts: 2,    // At least 2 photos to form a group
	}
}

// Cluster performs DBSCAN clustering on photos using their embeddings
func (d *DBSCAN) Cluster(embeddings map[int64][]float64) ClusteringResult {
	// Get all photo IDs
	ids := make([]int64, 0, len(embeddings))
	for id := range embeddings {
		ids = append(ids, id)
	}

	// Sort for deterministic results
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	n := len(ids)
	if n == 0 {
		return ClusteringResult{}
	}

	// Track cluster assignments: -1 = unvisited, 0 = noise, >0 = cluster ID
	labels := make(map[int64]int)
	for _, id := range ids {
		labels[id] = -1 // Unvisited
	}

	clusterID := 0

	// DBSCAN algorithm
	for _, id := range ids {
		if labels[id] != -1 {
			continue // Already processed
		}

		// Find neighbors
		neighbors := d.regionQuery(id, ids, embeddings)

		if len(neighbors) < d.MinPts {
			labels[id] = 0 // Mark as noise
			continue
		}

		// Start a new cluster
		clusterID++
		labels[id] = clusterID

		// Expand cluster
		seedSet := make([]int64, len(neighbors))
		copy(seedSet, neighbors)

		for i := 0; i < len(seedSet); i++ {
			neighborID := seedSet[i]

			if labels[neighborID] == 0 {
				// Change noise to border point
				labels[neighborID] = clusterID
			}

			if labels[neighborID] != -1 {
				continue // Already processed
			}

			labels[neighborID] = clusterID

			// Find neighbors of neighbor
			neighborNeighbors := d.regionQuery(neighborID, ids, embeddings)

			if len(neighborNeighbors) >= d.MinPts {
				// Add to seed set (expand cluster)
				for _, nn := range neighborNeighbors {
					if labels[nn] <= 0 { // Noise or unvisited
						seedSet = append(seedSet, nn)
					}
				}
			}
		}
	}

	// Build result
	return d.buildResult(ids, labels, embeddings, clusterID)
}

// regionQuery finds all points within eps distance of the target point
func (d *DBSCAN) regionQuery(targetID int64, allIDs []int64, embeddings map[int64][]float64) []int64 {
	var neighbors []int64
	targetEmb := embeddings[targetID]

	for _, id := range allIDs {
		if id == targetID {
			continue
		}

		distance := CosineDistance(targetEmb, embeddings[id])
		if distance <= d.Eps {
			neighbors = append(neighbors, id)
		}
	}

	return neighbors
}

// buildResult constructs the clustering result from labels
func (d *DBSCAN) buildResult(ids []int64, labels map[int64]int, embeddings map[int64][]float64, maxCluster int) ClusteringResult {
	result := ClusteringResult{
		Groups:    make([]PhotoGroup, 0),
		Ungrouped: make([]int64, 0),
	}

	// Group photos by cluster ID
	clusters := make(map[int][]int64)
	for _, id := range ids {
		label := labels[id]
		if label == 0 {
			result.Ungrouped = append(result.Ungrouped, id)
		} else if label > 0 {
			clusters[label] = append(clusters[label], id)
		}
	}

	// Convert to PhotoGroup slice
	for clusterID := 1; clusterID <= maxCluster; clusterID++ {
		photoIDs, exists := clusters[clusterID]
		if !exists || len(photoIDs) < d.MinPts {
			// Move small clusters to ungrouped
			result.Ungrouped = append(result.Ungrouped, photoIDs...)
			continue
		}

		// Calculate average pairwise similarity
		avgSim := d.calculateAvgSimilarity(photoIDs, embeddings)

		result.Groups = append(result.Groups, PhotoGroup{
			GroupID:       clusterID,
			PhotoIDs:      photoIDs,
			AvgSimilarity: avgSim,
		})
	}

	// Sort groups by size (largest first)
	sort.Slice(result.Groups, func(i, j int) bool {
		return len(result.Groups[i].PhotoIDs) > len(result.Groups[j].PhotoIDs)
	})

	// Renumber group IDs
	for i := range result.Groups {
		result.Groups[i].GroupID = i + 1
	}

	return result
}

// calculateAvgSimilarity calculates the average pairwise similarity within a group
func (d *DBSCAN) calculateAvgSimilarity(photoIDs []int64, embeddings map[int64][]float64) float64 {
	if len(photoIDs) < 2 {
		return 1.0
	}

	var totalSim float64
	var count int

	for i := 0; i < len(photoIDs); i++ {
		for j := i + 1; j < len(photoIDs); j++ {
			sim := CosineSimilarity(embeddings[photoIDs[i]], embeddings[photoIDs[j]])
			totalSim += sim
			count++
		}
	}

	if count == 0 {
		return 1.0
	}

	return totalSim / float64(count)
}

// ClusterPhotos is a convenience function to cluster photos with default settings
func ClusterPhotos(embeddings map[int64][]float64, similarityThreshold float64) ClusteringResult {
	dbscan := &DBSCAN{
		Eps:    1.0 - similarityThreshold, // Convert similarity to distance
		MinPts: 2,
	}
	return dbscan.Cluster(embeddings)
}

