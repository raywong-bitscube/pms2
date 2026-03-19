// Project Management System - Frontend JavaScript
console.log('PMS loaded');

// Notification helper
function showNotification(message, type) {
    const notification = document.createElement('div');
    notification.className = 'notification';
    notification.textContent = message;
    notification.style.cssText = 'position:fixed;top:20px;right:20px;padding:12px 24px;background:#1890ff;color:white;border-radius:4px;z-index:9999;animation:slideIn 0.3s;';
    document.body.appendChild(notification);
    setTimeout(() => notification.remove(), 3000);
}
