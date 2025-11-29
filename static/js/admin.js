// Mnemosyne Admin

const csrfToken = document.getElementById('csrfToken')?.value || '';
let confirmCallback = null;

document.addEventListener('DOMContentLoaded', () => {
    loadStats();
    loadUsers();
    setupConfirm();
});

async function loadStats() {
    try {
        const response = await fetch('/api/admin/stats');
        if (!response.ok) throw new Error('Failed');
        
        const stats = await response.json();
        document.getElementById('totalUsers').textContent = stats.total_users;
        document.getElementById('totalPhotos').textContent = stats.total_photos;
    } catch (error) {
        console.error('Error loading stats:', error);
    }
}

async function loadUsers() {
    const container = document.getElementById('usersList');

    try {
        const response = await fetch('/api/admin/users');
        if (!response.ok) throw new Error('Failed');

        const users = await response.json();
        
        if (!users?.length) {
            container.innerHTML = '<p style="color: var(--text-muted);">No users found</p>';
            return;
        }

        container.innerHTML = `
            <table class="table">
                <thead>
                    <tr>
                        <th>Username</th>
                        <th>Role</th>
                        <th>Photos</th>
                        <th>Joined</th>
                        <th></th>
                    </tr>
                </thead>
                <tbody>
                    ${users.map(user => `
                        <tr>
                            <td>${esc(user.username)}</td>
                            <td><span class="role-badge ${user.role}">${user.role}</span></td>
                            <td>${user.photo_count}</td>
                            <td>${formatDate(user.created_at)}</td>
                            <td>
                                <div class="table-actions">
                                    <button class="btn btn-ghost btn-sm" onclick="toggleRole(${user.id}, '${user.role}')">
                                        ${user.role === 'admin' ? 'Make User' : 'Make Admin'}
                                    </button>
                                    <button class="btn btn-danger btn-sm" onclick="confirmDelete(${user.id}, '${esc(user.username)}')">
                                        Delete
                                    </button>
                                </div>
                            </td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        `;
    } catch (error) {
        container.innerHTML = '<p style="color: var(--danger);">Failed to load users</p>';
    }
}

async function toggleRole(userId, currentRole) {
    const newRole = currentRole === 'admin' ? 'user' : 'admin';

    try {
        const response = await fetch(`/api/admin/users/${userId}/role`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrfToken
            },
            body: JSON.stringify({ role: newRole })
        });

        if (!response.ok) throw new Error(await response.text());
        loadUsers();
    } catch (error) {
        alert('Failed to update role');
    }
}

function confirmDelete(userId, username) {
    document.getElementById('confirmMessage').textContent = 
        `Delete user "${username}" and all their photos?`;
    document.getElementById('confirmModal').style.display = 'flex';
    confirmCallback = () => deleteUser(userId);
}

async function deleteUser(userId) {
    try {
        const response = await fetch(`/api/admin/users/${userId}`, {
            method: 'DELETE',
            headers: { 'X-CSRF-Token': csrfToken }
        });

        if (!response.ok) throw new Error(await response.text());
        
        loadUsers();
        loadStats();
    } catch (error) {
        alert('Failed to delete user');
    }
}

function setupConfirm() {
    const modal = document.getElementById('confirmModal');
    
    document.getElementById('confirmCancel')?.addEventListener('click', () => {
        modal.style.display = 'none';
        confirmCallback = null;
    });

    document.getElementById('confirmOk')?.addEventListener('click', () => {
        if (confirmCallback) confirmCallback();
        modal.style.display = 'none';
        confirmCallback = null;
    });

    modal?.addEventListener('click', (e) => {
        if (e.target === modal) {
            modal.style.display = 'none';
            confirmCallback = null;
        }
    });
}

function esc(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function formatDate(dateString) {
    return new Date(dateString).toLocaleDateString();
}
