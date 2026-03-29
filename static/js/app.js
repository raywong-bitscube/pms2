// Project Management System - Frontend JavaScript

// Mobile menu toggle
function toggleMenu() {
    var sidebar = document.getElementById('sidebar');
    var overlay = document.querySelector('.menu-overlay');
    if (sidebar && overlay) {
        sidebar.classList.toggle('open');
        overlay.classList.toggle('active');
    }
}

// Modal functions
function showCreateModal() { document.getElementById('createModal').style.display = 'flex'; }
function showUploadModal() { document.getElementById('uploadModal').style.display = 'flex'; }
function showUserModal() { document.getElementById('userModal').style.display = 'flex'; }
function closeModal(id) { document.getElementById(id).style.display = 'none'; }

// Project functions
async function createProject() {
    const data = {
        name: document.getElementById('newName').value,
        code: document.getElementById('newCode').value,
        template_id: parseInt(document.getElementById('newTemplate').value),
        start_date: document.getElementById('newStartDate').value,
        end_date: document.getElementById('newEndDate').value,
        description: document.getElementById('newDesc').value
    };
    
    const res = await fetch('/projects/api', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    });
    const result = await res.json();
    
    if (result.success) {
        window.location.href = '/projects';
    } else {
        alert('创建失败：' + result.message);
    }
}

function editProject(id, name, code, status, progress) {
    document.getElementById('editId').value = id;
    document.getElementById('editName').value = name;
    document.getElementById('editCode').value = code;
    document.getElementById('editStatus').value = status;
    document.getElementById('editProgress').value = progress;
    document.getElementById('editModal').style.display = 'flex';
}

async function saveProject() {
    const data = {
        id: parseInt(document.getElementById('editId').value),
        name: document.getElementById('editName').value,
        code: document.getElementById('editCode').value,
        status: parseInt(document.getElementById('editStatus').value),
        progress: parseInt(document.getElementById('editProgress').value),
        description: document.getElementById('editDesc').value
    };
    
    const res = await fetch('/projects/api', {
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    });
    const result = await res.json();
    
    if (result.success) {
        window.location.reload();
    } else {
        alert('保存失败：' + result.message);
    }
}

async function deleteProject(id) {
    if (!confirm('确定要删除这个项目吗？')) return;
    
    // 显示加载提示
    const btn = event.target;
    const originalText = btn.textContent;
    btn.disabled = true;
    btn.textContent = '删除中...';
    
    try {
        const res = await fetch('/projects/api', {
            method: 'DELETE',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({id})
        });
        const result = await res.json();
        
        if (result.success) {
            showNotification('删除成功', 'success');
            setTimeout(() => window.location.reload(), 500);
        } else {
            alert('删除失败：' + result.message);
            btn.disabled = false;
            btn.textContent = originalText;
        }
    } catch (e) {
        alert('网络错误：' + e.message);
        btn.disabled = false;
        btn.textContent = originalText;
    }
}

// Stage functions
async function updateStage(stageId, status, progress) {
    const newStatus = prompt('新的状态 (1=待开始，2=进行中，3=已完成):', status);
    if (newStatus === null) return;
    
    const newProgress = prompt('新的进度 (0-100):', progress);
    if (newProgress === null) return;
    
    const res = await fetch('/project/stages/api', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            stage_id: stageId,
            status: parseInt(newStatus),
            progress: parseInt(newProgress)
        })
    });
    const result = await res.json();
    
    if (result.success) {
        window.location.reload();
    } else {
        alert('更新失败：' + result.message);
    }
}

// Document functions
async function uploadDocument() {
    const form = document.getElementById('uploadForm');
    const formData = new FormData(form);
    
    const res = await fetch('/documents/api', {
        method: 'POST',
        body: formData
    });
    const result = await res.json();
    
    if (result.success) {
        window.location.reload();
    } else {
        alert('上传失败：' + result.message);
    }
}

async function deleteDocument(id) {
    if (!confirm('确定要删除这个文档吗？')) return;
    
    const res = await fetch('/documents/api', {
        method: 'DELETE',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({id})
    });
    const result = await res.json();
    
    if (result.success) {
        window.location.reload();
    } else {
        alert('删除失败：' + result.message);
    }
}

// User functions
async function createUser() {
    const data = {
        username: document.getElementById('newUsername').value,
        password: document.getElementById('newPassword').value,
        real_name: document.getElementById('newRealName').value,
        email: document.getElementById('newEmail').value
    };
    
    const res = await fetch('/users/api', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    });
    const result = await res.json();
    
    if (result.success) {
        window.location.reload();
    } else {
        alert('创建失败：' + result.message);
    }
}

// Notification
function showNotification(message, type = 'info') {
    const colors = {info: '#1890ff', success: '#52c41a', error: '#f5222d'};
    const notification = document.createElement('div');
    notification.textContent = message;
    notification.style.cssText = `position:fixed;top:20px;right:20px;padding:12px 24px;background:${colors[type]};color:white;border-radius:4px;z-index:9999;animation:slideIn 0.3s;`;
    document.body.appendChild(notification);
    setTimeout(() => notification.remove(), 3000);
}

// Close modal when clicking outside
window.onclick = function(event) {
    if (event.target.classList.contains('modal')) {
        event.target.style.display = 'none';
    }
}
