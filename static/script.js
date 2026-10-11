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

async function updateDashboard() {
    try {
        const res = await fetch('/api/data');
        const data = await res.json();

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
        if (data.agents && Object.keys(data.agents).length > 0) {
            agentBody.innerHTML = '';
            Object.values(data.agents).forEach(a => {
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

        data.devices.forEach(d => {
            const statusClass = d.status.startsWith('Fehlt') ? 'status-Fehlt' : 'status-Online';
            const latencyText = d.status.startsWith('Fehlt') ? '-' : d.latency_ms + ' ms';
            const jitterText = d.status.startsWith('Fehlt') ? '-' : d.jitter_ms + ' ms';

            let ipDisplay = d.ip;

            if ((d.port.includes('Web') || d.port.includes('UPnP')) && !d.status.startsWith('Fehlt')) {
                let scheme = d.port.includes('HTTPS') ? 'https://' : 'http://';
                let linkUrl = scheme + d.ip;
                if (d.port.includes('Alt')) {
                    linkUrl = scheme + d.ip + ':8080';
                }
                ipDisplay = '<a href="' + linkUrl + '" target="_blank" style="color: #4caf50; text-decoration: none;" title="Webinterface öffnen">🔗 ' + d.ip + '</a>';
            }

            let macDisplay = (d.mac && d.mac !== "") ? d.mac : '-';

            tbody.innerHTML += '<tr>' +
                '<td class="' + statusClass + '">' + d.status + '</td>' +
                '<td>' + ipDisplay + '</td>' +
                '<td class="mac-text">' + macDisplay + '</td>' +
                '<td>' + d.hostname + '</td>' +
                '<td>' + d.port + '</td>' +
                '<td>' + latencyText + '</td>' +
                '<td>' + jitterText + '</td>' +
            '</tr>';
        });
    } catch (err) {
        console.error("Fehler beim Abrufen der Daten:", err);
    }
}

setInterval(updateDashboard, 5000);
updateDashboard();
