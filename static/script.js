let ws;
let reconnectInterval = 3000;

async function logout() {
    await fetch('/logout', { method: 'POST' });
    window.location.href = '/login';
}

async function updateStatus() {
    try {
        const response = await fetch('/api/status');
        if (response.status === 401) {
            window.location.href = '/login';
            return;
        }
        const data = await response.json();
        const statusBadge = document.getElementById('container-status-badge');
        
        statusBadge.textContent = data.status || 'Unknown';
        
        const btnStart = document.getElementById('btn-start');
        const btnStop = document.getElementById('btn-stop');
        const btnRestart = document.getElementById('btn-restart');

        if (data.status === 'running') {
            statusBadge.className = 'px-2 py-1 rounded-md text-[10px] font-black uppercase tracking-tighter bg-green-500/20 text-green-500 border border-green-500/30';
            btnStart.disabled = true;
            btnStart.classList.add('opacity-50', 'cursor-not-allowed');
            btnStop.disabled = false;
            btnStop.classList.remove('opacity-50', 'cursor-not-allowed');
        } else {
            statusBadge.className = 'px-2 py-1 rounded-md text-[10px] font-black uppercase tracking-tighter bg-red-500/20 text-red-500 border border-red-500/30';
            btnStart.disabled = false;
            btnStart.classList.remove('opacity-50', 'cursor-not-allowed');
            btnStop.disabled = true;
            btnStop.classList.add('opacity-50', 'cursor-not-allowed');
        }

        document.getElementById('container-id').textContent = data.id || '--------';
        document.getElementById('container-image').textContent = data.image || 'N/A';

        // Update handling
        const updateBadge = document.getElementById('update-status-badge');
        const lastCheck = document.getElementById('last-check-time');
        const progressContainer = document.getElementById('update-progress-container');
        const progressBar = document.getElementById('update-progress-bar');
        const progressText = document.getElementById('update-progress-text');
        const btnCheck = document.getElementById('btn-check-update');

        // Update versions
        const currentSha = document.getElementById('current-version-sha');
        const latestSha = document.getElementById('latest-version-sha');
        
        if (data.imageId) {
            currentSha.textContent = data.imageId.replace('sha256:', '').substring(0, 12);
        }

        if (data.updateStatus) {
            const us = data.updateStatus;
            const checkDate = new Date(us.lastCheck);
            lastCheck.textContent = checkDate.getFullYear() > 2000 ? checkDate.toLocaleString() : 'Never';
            
            if (us.latestVersion) {
                latestSha.textContent = us.latestVersion.replace('sha256:', '').substring(0, 12);
                
                // Highlight matches
                const currentId = data.imageId || us.currentVersion;
                if (currentId === us.latestVersion) {
                    currentSha.classList.remove('text-gray-500');
                    currentSha.classList.add('text-green-500');
                    latestSha.classList.remove('text-gray-500');
                    latestSha.classList.add('text-green-500');
                } else {
                    currentSha.classList.add('text-gray-500');
                    currentSha.classList.remove('text-green-500');
                    latestSha.classList.add('text-gray-500');
                    latestSha.classList.remove('text-green-500');
                }
            }

            if (us.isUpdating) {
                updateBadge.textContent = 'Updating';
                updateBadge.className = 'px-2 py-1 rounded-md text-[10px] font-black uppercase tracking-tighter bg-blue-500/20 text-blue-500 border border-blue-500/30';
                progressContainer.classList.remove('hidden');
                progressBar.style.width = '100%'; 
                progressText.textContent = us.progress || 'Updating...';
                btnCheck.disabled = true;
                btnCheck.classList.add('opacity-50', 'cursor-not-allowed');
            } else if (us.isPending) {
                const pendingDate = new Date(us.pendingTime);
                updateBadge.textContent = 'Pending';
                updateBadge.className = 'px-2 py-1 rounded-md text-[10px] font-black uppercase tracking-tighter bg-purple-500/20 text-purple-500 border border-purple-500/30';
                progressContainer.classList.remove('hidden');
                progressBar.style.width = '100%';
                progressText.textContent = `Scheduled for ${pendingDate.toLocaleTimeString()}`;
                btnCheck.disabled = true;
                btnCheck.classList.add('opacity-50', 'cursor-not-allowed');
            } else if (us.isChecking) {
                updateBadge.textContent = 'Checking';
                updateBadge.className = 'px-2 py-1 rounded-md text-[10px] font-black uppercase tracking-tighter bg-yellow-500/20 text-yellow-500 border border-yellow-500/30';
                progressContainer.classList.add('hidden');
                btnCheck.disabled = true;
                btnCheck.classList.add('opacity-50', 'cursor-not-allowed');
            } else {
                updateBadge.textContent = 'Idle';
                updateBadge.className = 'px-2 py-1 rounded-md text-[10px] font-black uppercase tracking-tighter bg-gray-800';
                progressContainer.classList.add('hidden');
                btnCheck.disabled = false;
                btnCheck.classList.remove('opacity-50', 'cursor-not-allowed');
            }

            if (us.error) {
                progressText.textContent = us.error;
                progressText.classList.add('text-red-500');
                progressContainer.classList.remove('hidden');
                progressBar.classList.add('bg-red-500');
            } else {
                progressText.classList.remove('text-red-500');
                progressBar.classList.remove('bg-red-500');
            }
        }
    } catch (e) {
        console.error('Status fetch error', e);
    }
}

async function checkUpdate() {
    try {
        const response = await fetch('/api/check-update', { method: 'POST' });
        if (response.ok) {
            updateStatus();
        }
    } catch (e) {
        console.error('Update check error', e);
    }
}

function confirmAction(type) {
    const modal = document.getElementById('modal-container');
    const title = document.getElementById('modal-title');
    const desc = document.getElementById('modal-desc');
    const icon = document.getElementById('modal-icon');
    const confirmBtn = document.getElementById('modal-confirm-btn');
    
    let colorClass = "";
    let iconSvg = "";
    
    switch(type) {
        case 'start':
            title.textContent = "Start Instance";
            desc.textContent = "Initialize the Soulmask container and bring systems online.";
            colorClass = "bg-green-600";
            iconSvg = `<svg class="w-8 h-8 text-green-500" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd" /></svg>`;
            break;
        case 'restart':
            title.textContent = "Restart Instance";
            desc.textContent = "Recycle the active process. This will temporarily drop all connections.";
            colorClass = "bg-blue-600";
            iconSvg = `<svg class="w-8 h-8 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>`;
            break;
        case 'stop':
            title.textContent = "Stop Instance";
            desc.textContent = "Terminate the container process. All active sessions will be killed.";
            colorClass = "bg-red-600";
            iconSvg = `<svg class="w-8 h-8 text-red-500" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd" /></svg>`;
            break;
    }
    
    icon.innerHTML = iconSvg;
    confirmBtn.className = `flex-1 px-4 py-3 rounded-xl text-xs font-bold transition-all shadow-lg active:scale-95 uppercase tracking-widest text-white ${colorClass}`;
    confirmBtn.onclick = () => executeAction(type);
    
    modal.classList.remove('hidden');
}

function closeModal() {
    document.getElementById('modal-container').classList.add('hidden');
}

async function executeAction(name) {
    closeModal();
    try {
        const response = await fetch(`/api/action/${name}`, { method: 'POST' });
        if (!response.ok) {
            const err = await response.text();
            alert('Action failed: ' + err);
        }
        updateStatus();
    } catch (e) {
        alert('Action failed: ' + e);
    }
}

function connectLogs() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/api/logs`);
    const terminal = document.getElementById('terminal');
    const connStatus = document.getElementById('connection-status');

    ws.onopen = () => {
        const div = document.createElement('div');
        div.className = 'text-green-500 font-bold border-l-2 border-green-500 pl-2 my-2 animate-pulse';
        div.textContent = `[${new Date().toLocaleTimeString()}] UPLINK_ESTABLISHED: STREAM_CONNECTED (NEWEST AT TOP)`;
        terminal.prepend(div);
        connStatus.innerHTML = '<div class="w-2 h-2 rounded-full bg-green-500 animate-pulse"></div><span class="text-xs font-medium text-gray-300 uppercase tracking-tighter">Live Stream Active</span>';
    };

    ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            const line = document.createElement('div');
            line.className = 'hover:bg-white/5 transition-colors border-l-2 border-transparent pl-2 py-0.5 flex items-start group animate-zoom';
            
            if (data.type === 'stderr') {
                line.classList.add('bg-red-500/5', 'border-red-500/30');
            }

            const timeSpan = document.createElement('span');
            timeSpan.className = 'text-gray-600 mr-3 text-[10px] select-none font-mono shrink-0 pt-0.5';
            timeSpan.textContent = new Date().toLocaleTimeString([], {hour12: false});
            
            const contentSpan = document.createElement('span');
            contentSpan.className = data.type === 'stderr' ? 'text-red-400 font-mono text-sm' : 'text-gray-300 font-mono text-sm';
            contentSpan.textContent = data.content;
            
            line.appendChild(timeSpan);
            line.appendChild(contentSpan);
            terminal.prepend(line);
            
            if (terminal.childNodes.length > 2000) {
                terminal.removeChild(terminal.lastChild);
            }
        } catch (e) {
            // Fallback for raw text
            const line = document.createElement('div');
            line.className = 'text-gray-500 text-xs italic opacity-50';
            line.textContent = event.data;
            terminal.prepend(line);
        }
    };

    ws.onclose = () => {
        connStatus.innerHTML = '<div class="w-2 h-2 rounded-full bg-red-500"></div><span class="text-xs font-medium text-gray-500 uppercase tracking-tighter">Stream Offline</span>';
        setTimeout(connectLogs, reconnectInterval);
    };
}

function clearLogs() {
    document.getElementById('terminal').innerHTML = '';
}

// Initialization
updateStatus();
setInterval(updateStatus, 5000);
connectLogs();
