const navigationLinks = document.querySelectorAll('.main-navigation a');
const pageSections = document.querySelectorAll('.page-section');

function createMeshNode(node) {
    const item = document.createElement('li');
    const card = document.createElement('div');
    card.className = 'mesh-node' + (node.active ? ' mesh-node-active' : ' mesh-node-inactive');

    const heading = document.createElement('div');
    heading.className = 'mesh-node-heading';
    const icon = node.device_type === 'router' ? '🌐' :
        node.device_type === 'repeater' ? '📶' : '💻';
    const name = document.createElement('strong');
    name.textContent = `${icon} ${node.name || 'Unbekanntes Gerät'}`;
    heading.appendChild(name);

    const status = document.createElement('span');
    status.className = 'mesh-node-status';
    status.textContent = node.active ? 'Online' : 'Offline';
    heading.appendChild(status);
    card.appendChild(heading);

    const details = [node.ip, node.mac].filter(Boolean);
    if (details.length > 0) {
        const info = document.createElement('span');
        info.className = 'mesh-node-details';
        info.textContent = details.join(' · ');
        card.appendChild(info);
    }
    item.appendChild(card);

    if (Array.isArray(node.children) && node.children.length > 0) {
        const children = document.createElement('ul');
        node.children.forEach(child => children.appendChild(createMeshNode(child)));
        item.appendChild(children);
    }
    return item;
}

function updateMeshView(meshTree, lastUpdated) {
    const container = document.getElementById('meshTree');
    const updated = document.getElementById('meshUpdated');
    updated.textContent = lastUpdated ? `Zuletzt aktualisiert: ${lastUpdated}` : '';

    if (!meshTree) {
        container.innerHTML = '<p class="empty-state">Noch keine Mesh-Topologie verfügbar. Prüfe die Verbindung zur FRITZ!Box.</p>';
        return;
    }

    const tree = document.createElement('ul');
    tree.className = 'mesh-tree-root';
    tree.appendChild(createMeshNode(meshTree));
    container.replaceChildren(tree);
}

function formatMilliseconds(value) {
    const number = Number(value);
    return Number.isFinite(number) && number > 0 ? `${number.toFixed(1)} ms` : 'Keine Messung';
}

function updateInternetView(data, agents) {
    const serverLatency = Number(data.current_wan_latency);
    const serverJitter = Number(data.current_wan_jitter);
    document.getElementById('serverLatency').textContent = formatMilliseconds(serverLatency);
    document.getElementById('serverJitter').textContent =
        Number.isFinite(serverLatency) && serverLatency > 0 && Number.isFinite(serverJitter)
            ? `${serverJitter.toFixed(1)} ms`
            : 'Keine Messung';

    const agentEntries = Object.values(agents);
    const measuredAgents = agentEntries.filter(agent => Number(agent.latency_ms) > 0);
    const averageLatency = measuredAgents.reduce((sum, agent) => sum + Number(agent.latency_ms), 0) / (measuredAgents.length || 1);
    const averageJitter = measuredAgents.reduce((sum, agent) => sum + (Number(agent.jitter_ms) || 0), 0) / (measuredAgents.length || 1);
    document.getElementById('agentLatency').textContent =
        measuredAgents.length ? `${averageLatency.toFixed(1)} ms` : 'Keine Messung';
    document.getElementById('agentJitter').textContent =
        measuredAgents.length ? `${averageJitter.toFixed(1)} ms` : 'Keine Messung';
    document.getElementById('agentMeasurements').textContent = measuredAgents.length;
    document.getElementById('agentMeasurementsTotal').textContent = agentEntries.length;

    const table = document.getElementById('internetAgentTable');
    table.replaceChildren();
    if (agentEntries.length === 0) {
        const row = table.insertRow();
        const cell = row.insertCell();
        cell.colSpan = 6;
        cell.className = 'empty-state';
        cell.textContent = 'Noch keine Agenten verbunden.';
        return;
    }

    agentEntries
        .sort((a, b) => (Number(a.latency_ms) || Infinity) - (Number(b.latency_ms) || Infinity))
        .forEach(agent => {
            const row = table.insertRow();
            [
                agent.hostname || 'Unbekannter Agent',
                agent.ip || '–',
                agent.repeater_name || agent.access_point || 'LAN / Unbekannt',
                formatMilliseconds(agent.latency_ms),
                Number(agent.latency_ms) > 0 && Number.isFinite(Number(agent.jitter_ms))
                    ? `${Number(agent.jitter_ms).toFixed(1)} ms`
                    : 'Keine Messung',
                agent.last_updated || '–'
            ].forEach(value => {
                row.insertCell().textContent = value;
            });
        });
}

let callerUpdateInProgress = false;

function renderCallerDetails(element, fields) {
    element.replaceChildren();
    fields.forEach(([label, value]) => {
        const term = document.createElement('dt');
        term.textContent = label;
        const description = document.createElement('dd');
        description.textContent = value || '–';
        element.append(term, description);
    });
}

async function updateCallerView() {
    if (callerUpdateInProgress) {
        return;
    }
    callerUpdateInProgress = true;

    const ipElement = document.getElementById('callerIP');
    const message = document.getElementById('callerMessage');
    const deviceCard = document.getElementById('callerDeviceCard');
    const agentCard = document.getElementById('callerAgentCard');
    const meshMessage = document.getElementById('callerMeshMessage');
    const meshPath = document.getElementById('callerMeshPath');
    try {
        const response = await fetch('/api/caller');
        if (!response.ok) {
            throw new Error(`Aufrufer-API fehlgeschlagen: HTTP ${response.status}`);
        }
        const data = await response.json();
        ipElement.textContent = data.ip || 'Nicht ermittelbar';

        const device = data.device;
        const agent = data.agent;
        meshPath.replaceChildren();
        deviceCard.hidden = !device;
        agentCard.hidden = !agent;

        if (device) {
            renderCallerDetails(document.getElementById('callerDeviceDetails'), [
                ['Hostname', device.hostname],
                ['Status', device.status],
                ['MAC-Adresse', device.mac],
                ['Erkannter Dienst', device.port],
                ['Verbunden via', [device.mesh_icon, device.connected_via].filter(Boolean).join(' ')],
                ['Latenz', formatMilliseconds(device.latency_ms)],
                ['Jitter', formatMilliseconds(device.jitter_ms)]
            ]);
        }

        if (agent) {
            renderCallerDetails(document.getElementById('callerAgentDetails'), [
                ['Hostname', agent.hostname],
                ['Repeater', agent.repeater_name],
                ['Zugangspunkt', agent.access_point],
                ['Internet-Latenz', formatMilliseconds(agent.latency_ms)],
                ['Jitter', formatMilliseconds(agent.jitter_ms)],
                ['WLAN-Signal', Number(agent.wifi_rssi_dbm) < 0 ? `${agent.wifi_rssi_dbm} dBm` : 'Keine WLAN-Messung'],
                ['Agent-Version', agent.version],
                ['Letztes Update', agent.last_updated]
            ]);
        }

        if (Array.isArray(data.mesh_path) && data.mesh_path.length > 0) {
            data.mesh_path.forEach(node => {
                const item = document.createElement('li');
                const card = document.createElement('div');
                card.className = 'caller-mesh-node';

                const typeIcon = node.device_type === 'router' ? '🌐' :
                    node.device_type === 'repeater' ? '📶' : '💻';
                const name = document.createElement('strong');
                name.textContent = `${typeIcon} ${node.name || 'Unbekanntes Gerät'}`;
                card.appendChild(name);

                const details = [node.ip, node.mac].filter(Boolean);
                if (details.length > 0) {
                    const info = document.createElement('span');
                    info.textContent = details.join(' · ');
                    card.appendChild(info);
                }
                item.appendChild(card);
                meshPath.appendChild(item);
            });
            meshMessage.textContent = 'Verbindungspfad vom Router bis zum erkannten Aufrufer.';
            meshMessage.classList.remove('empty-state');
        } else {
            meshMessage.textContent = 'Für diesen Aufrufer wurde kein Pfad in der Mesh-Topologie gefunden.';
            meshMessage.classList.add('empty-state');
        }

        message.textContent = device || agent
            ? 'Der Aufrufer wurde in den erkannten Netzwerkdaten gefunden.'
            : 'Für diese IP-Adresse wurde noch kein Gerät und kein Agent erkannt.';
        message.classList.toggle('empty-state', !device && !agent);
    } catch (error) {
        message.textContent = 'Die Daten des Aufruferknotens konnten nicht geladen werden.';
        message.classList.add('empty-state');
        meshMessage.textContent = 'Der Mesh-Pfad konnte nicht geladen werden.';
        meshMessage.classList.add('empty-state');
        console.error('Fehler beim Abrufen der Aufruferdaten:', error);
    } finally {
        callerUpdateInProgress = false;
    }
}

function showSection(sectionId) {
    let section = document.getElementById(sectionId);
    if (!section || !section.classList.contains('page-section')) {
        section = document.getElementById('overview');
    }

    pageSections.forEach(page => {
        page.hidden = page !== section;
    });
    navigationLinks.forEach(link => {
        if (link.hash === '#' + sectionId) {
            link.setAttribute('aria-current', 'page');
        } else {
            link.removeAttribute('aria-current');
        }
    });

    if (sectionId === 'overview') {
        deviceChart.resize();
        latencyChart.resize();
    }
}

navigationLinks.forEach(link => {
    link.addEventListener('click', event => {
        event.preventDefault();
        const sectionId = link.hash.slice(1);
        if (window.location.hash !== link.hash) {
            window.history.pushState(null, '', link.hash);
        }
        showSection(sectionId);
        if (sectionId === 'caller') {
            updateCallerView();
        }
    });
});

window.addEventListener('popstate', () => {
    const sectionId = window.location.hash.slice(1) || 'overview';
    showSection(sectionId);
    if (sectionId === 'caller') {
        updateCallerView();
    }
});

const ctxDevice = document.getElementById('deviceChart').getContext('2d');
const ctxLatency = document.getElementById('latencyChart').getContext('2d');

Chart.defaults.color = '#e0e0e0';
Chart.defaults.borderColor = '#333';

const deviceChart = new Chart(ctxDevice, {
    type: 'line',
    data: {
        labels: [],
        datasets: [
            { label: 'Geräte Online', data: [], borderColor: '#4caf50', backgroundColor: 'rgba(76, 175, 80, 0.1)', fill: true, tension: 0.3, yAxisID: 'y' },
            { label: 'Internet-Latenz (ms)', data: [], borderColor: '#ff9800', backgroundColor: 'rgba(255, 152, 0, 0.0)', borderDash: [5,5], fill: false, tension: 0.3, yAxisID: 'y1' }
        ]
    },
    options: {
        responsive: true,
        animation: { duration: 0 },
        scales: {
            y: { type: 'linear', display: true, position: 'left', beginAtZero: true, title: {display: true, text: 'Geräte'} },
            y1: { type: 'linear', display: true, position: 'right', beginAtZero: true, grid: {drawOnChartArea: false}, title: {display: true, text: 'ms'} }
        }
    }
});

const latencyChart = new Chart(ctxLatency, {
    type: 'line',
    data: {
        labels: [],
        datasets: [
            { label: 'Ø Lokale Latenz (ms)', data: [], borderColor: '#2196f3', backgroundColor: 'rgba(33, 150, 243, 0.1)', fill: true, tension: 0.3 },
            { label: 'Ø Internet-Latenz (WAN)', data: [], borderColor: '#e91e63', backgroundColor: 'rgba(233, 30, 99, 0.1)', fill: true, tension: 0.3 }
        ]
    },
    options: { responsive: true, animation: { duration: 0 }, scales: { y: { beginAtZero: true } } }
});

showSection(window.location.hash.slice(1) || 'overview');
if (window.location.hash === '#caller') {
    updateCallerView();
}

async function updateDashboard() {
    try {
        const res = await fetch('/api/data');
        if (!res.ok) {
            throw new Error(`API-Anfrage fehlgeschlagen: HTTP ${res.status}`);
        }
        const data = await res.json();
        updateMeshView(data.mesh_tree, data.last_mesh_update);
        const devices = Array.isArray(data.devices) ? data.devices : [];
        const agents = data.agents && typeof data.agents === 'object' ? data.agents : {};
        document.getElementById('deviceCount').textContent = devices.length;
        document.getElementById('agentCount').textContent = Object.keys(agents).length;
        updateInternetView(data, agents);

        const labels = data.history.map(h => h.time);
        const activeData = data.history.map(h => h.active);
        const wanLatencyData = data.history.map(h => h.wan_latency_ms);
        const localLatencyData = data.history.map(h => h.avg_latency);

        deviceChart.data.labels = labels;
        deviceChart.data.datasets[0].data = activeData;
        deviceChart.data.datasets[1].data = wanLatencyData;
        deviceChart.update();

        latencyChart.data.labels = labels;
        latencyChart.data.datasets[0].data = localLatencyData;
        latencyChart.data.datasets[1].data = wanLatencyData;
        latencyChart.update();

        const agentBody = document.getElementById('agentTable');
        if (Object.keys(agents).length > 0) {
            agentBody.innerHTML = '';
            Object.values(agents).forEach(a => {
                let signalText = '🔌 LAN';
                if (a.wifi_rssi_dbm < 0) {
                    if (a.wifi_rssi_dbm >= -65) {
                        signalText = '🟢 ' + a.wifi_rssi_dbm + ' dBm';
                    } else if (a.wifi_rssi_dbm >= -75) {
                        signalText = '🟡 ' + a.wifi_rssi_dbm + ' dBm';
                    } else {
                        signalText = '🔴 ' + a.wifi_rssi_dbm + ' dBm';
                    }
                }

                let repeaterInfo = a.repeater_name ? a.repeater_name : '🔌 LAN';
                let versionInfo = a.version ? 'v' + a.version : 'v1.0';

                agentBody.innerHTML += '<tr>' +
                    '<td style="color:#888;">' + a.last_updated + '</td>' +
                    '<td>' + a.ip + '</td>' +
                    '<td>' + a.hostname + '</td>' +
                    '<td><span style="color:#888;">' + versionInfo + '</span></td>' +
                    '<td><span class="repeater-badge">' + repeaterInfo + '</span></td>' +
                    '<td>' + a.latency_ms + ' ms</td>' +
                    '<td>' + a.jitter_ms + ' ms</td>' +
                    '<td>' + signalText + '</td>' +
                '</tr>';
            });
        } else {
            agentBody.innerHTML = '<tr><td colspan="8" style="color: #888; font-style: italic;">Warte auf Agenten (Port 8081)...</td></tr>';
        }

        const tbody = document.getElementById('deviceTable');
        tbody.innerHTML = '';

        devices.forEach(d => {
            const statusClass = d.status.startsWith('Fehlt') ? 'status-Fehlt' : 'status-Online';
            const latencyText = d.status.startsWith('Fehlt') ? '-' : d.latency_ms + ' ms';
            const jitterText = d.status.startsWith('Fehlt') ? '-' : d.jitter_ms + ' ms';

            const connectedVia = d.connected_via || d.connectedVia || 'Unbekannt';
            const meshIcon = d.mesh_icon || '❓';
            const portValue = d.port || 'Unbekannt';

            let ipDisplay = d.ip;
            if ((portValue.includes('Web') || portValue.includes('UPnP')) && !d.status.startsWith('Fehlt')) {
                let scheme = portValue.includes('HTTPS') ? 'https://' : 'http://';
                let linkUrl = scheme + d.ip;
                if (portValue.includes('Alt')) {
                    linkUrl = scheme + d.ip + ':8080';
                }
                ipDisplay = '<a href="' + linkUrl + '" target="_blank" style="color: #4caf50; text-decoration: none;" title="Webinterface öffnen">🔗 ' + d.ip + '</a>';
            }

            let macDisplay = (d.mac && d.mac !== "") ? d.mac : '-';
            let meshDisplay = connectedVia === 'Unbekannt' ? '❓ Unbekannt' : meshIcon + ' ' + connectedVia;
            let meshClass = connectedVia === 'Unbekannt' ? 'mesh-unknown' : 'mesh-connected';

            tbody.innerHTML += '<tr>' +
                '<td class="' + statusClass + '">' + d.status + '</td>' +
                '<td>' + ipDisplay + '</td>' +
                '<td class="mac-text">' + macDisplay + '</td>' +
                '<td>' + d.hostname + '</td>' +
                '<td class="' + meshClass + '">' + meshDisplay + '</td>' +
                '<td>' + portValue + '</td>' +
                '<td>' + latencyText + '</td>' +
                '<td>' + jitterText + '</td>' +
            '</tr>';
        });

        if (!document.getElementById('caller').hidden) {
            updateCallerView();
        }
    } catch (err) {
        console.error("Fehler beim Abrufen der Daten:", err);
    }
}

setInterval(updateDashboard, 5000);
updateDashboard();
