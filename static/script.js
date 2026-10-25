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

function findMeshPath(node, targetIP) {
    if (!node || !targetIP) {
        return null;
    }
    if (node.ip === targetIP) {
        return [node];
    }

    for (const child of node.children || []) {
        const childPath = findMeshPath(child, targetIP);
        if (childPath) {
            return [node, ...childPath];
        }
    }
    return null;
}

function appendAgentMeshPath(tableBody, agent, meshTree) {
    const row = tableBody.insertRow();
    row.className = 'agent-mesh-path-row';
    const cell = row.insertCell();
    cell.colSpan = 11;

    const label = document.createElement('strong');
    label.textContent = 'Mesh-Pfad: ';
    cell.appendChild(label);

    const path = findMeshPath(meshTree, agent.ip);
    if (!path) {
        const unavailable = document.createElement('span');
        unavailable.className = 'agent-mesh-path-unavailable';
        unavailable.textContent = meshTree
            ? 'Für diese Agent-IP nicht in der Mesh-Topologie gefunden'
            : 'Mesh-Topologie nicht verfügbar';
        cell.appendChild(unavailable);
        return;
    }

    path.forEach((node, index) => {
        if (index > 0) {
            const separator = document.createElement('span');
            separator.className = 'agent-mesh-path-separator';
            separator.setAttribute('aria-hidden', 'true');
            separator.textContent = '›';
            cell.appendChild(separator);
        }

        const entry = document.createElement('span');
        entry.className = 'agent-mesh-path-node';
        const icon = node.device_type === 'router' ? '🌐' :
            node.device_type === 'repeater' ? '📶' : '💻';
        entry.textContent = `${icon} ${node.name || node.ip || 'Unbekannter Knoten'}`;
        cell.appendChild(entry);
    });
}

function formatMilliseconds(value) {
    const number = Number(value);
    return Number.isFinite(number) && number > 0 ? `${number.toFixed(1)} ms` : 'Keine Messung';
}

const measurementFreshnessLimitMs = 90 * 1000;

function getMeasurementFreshness(timestamp) {
    const measuredAt = Date.parse(timestamp || '');
    if (!Number.isFinite(measuredAt)) {
        return { known: false, fresh: false, ageMs: null };
    }
    const ageMs = Math.max(0, Date.now() - measuredAt);
    return { known: true, fresh: ageMs <= measurementFreshnessLimitMs, ageMs };
}

function formatMeasurementAge(timestamp) {
    const freshness = getMeasurementFreshness(timestamp);
    if (!freshness.known) {
        return 'Alter unbekannt';
    }
    const seconds = Math.floor(freshness.ageMs / 1000);
    if (seconds < 60) {
        return `vor ${seconds} s`;
    }
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) {
        return `vor ${minutes} min`;
    }
    return `vor ${Math.floor(minutes / 60)} h`;
}

function formatMeasurementValue(value, metric, timestamp, latestProbeSuccesses) {
    const freshness = getMeasurementFreshness(timestamp);
    const number = Number(value);
    const valid = Number.isFinite(number) && (number > 0 || (metric === 'jitter' && number === 0));
    if (!valid || !timestamp) {
        return { text: 'Keine aktuelle Messung', available: false };
    }
    const reading = `${number.toFixed(1)} ms`;
    if (!freshness.fresh) {
        return { text: `${reading} (veraltet)`, available: false };
    }
    if (Number.isFinite(Number(latestProbeSuccesses)) && Number(latestProbeSuccesses) === 0) {
        return { text: `${reading} (letzte Prüfung ohne Erfolg)`, available: false };
    }
    return { text: reading, available: true };
}

function formatProbeSuccesses(successes, attempts) {
    const successful = Number(successes);
    const total = Number(attempts);
    if (!Number.isFinite(successful) || !Number.isFinite(total) || total <= 0) {
        return 'Keine Versuche';
    }
    return `${successful}/${total} (${Math.round(successful / total * 100)} %)`;
}

function getMetricQuality(metric, value, available = true) {
    const number = typeof value === 'number' ? value : Number.parseFloat(value);
    if (!available || !Number.isFinite(number)) {
        return { label: 'Keine Messung', level: 'unavailable' };
    }

    if (metric === 'wifi') {
        if (number >= -60) return { label: 'Gut', level: 'good' };
        if (number >= -70) return { label: 'Mittel', level: 'medium' };
        if (number >= -80) return { label: 'Schwach', level: 'weak' };
        return { label: 'Schlecht', level: 'poor' };
    }
    if (metric === 'latency') {
        if (number <= 50) return { label: 'Gut', level: 'good' };
        if (number <= 100) return { label: 'Mittel', level: 'medium' };
        return { label: 'Schlecht', level: 'poor' };
    }
    if (metric === 'jitter') {
        if (number <= 10) return { label: 'Gut', level: 'good' };
        if (number <= 30) return { label: 'Mittel', level: 'medium' };
        return { label: 'Schlecht', level: 'poor' };
    }
    return { label: 'Keine Einschätzung', level: 'unavailable' };
}

function getMetricGuidance(metric) {
    if (metric === 'latency') return 'Latenz: gut bis 50 ms, mittel bis 100 ms, darüber schlecht';
    if (metric === 'jitter') return 'Jitter: gut bis 10 ms, mittel bis 30 ms, darüber schlecht';
    if (metric === 'wifi') return 'WLAN-RSSI: gut ab -60 dBm, mittel ab -70 dBm, schwach ab -80 dBm, darunter schlecht';
    return '';
}

function createQualityDisplay(value, metric, available = true) {
    const display = document.createElement('span');
    display.className = 'quality-display';

    const reading = document.createElement('span');
    reading.className = 'quality-reading';
    reading.textContent = value;
    display.appendChild(reading);

    const quality = getMetricQuality(metric, value, available);
    const indicator = document.createElement('span');
    indicator.className = `quality-indicator quality-${quality.level}`;
    indicator.textContent = quality.label === 'Gut' ? '● Gut' :
        quality.label === 'Mittel' ? '● Mittel' :
        quality.label === 'Schwach' ? '● Schwach' :
        quality.label === 'Schlecht' ? '● Schlecht' : '● Keine Messung';
    indicator.setAttribute('aria-label', `Güte: ${quality.label}`);
    indicator.title = getMetricGuidance(metric);
    display.appendChild(indicator);
    return display;
}

function setQualityDisplay(element, value, metric, available = true) {
    element.replaceChildren(createQualityDisplay(value, metric, available));
}

const accessPointMetricHistory = new Map();
const maxAccessPointHistorySamples = 60;

function appendMetricSample(accessPoint, metric, value) {
    const key = `${accessPoint}:${metric}`;
    if (!accessPointMetricHistory.has(key)) {
        accessPointMetricHistory.set(key, []);
    }
    const samples = accessPointMetricHistory.get(key);
    if (samples.length > 0 && Object.is(samples[samples.length - 1], value)) {
        return;
    }
    samples.push(value);
    if (samples.length > maxAccessPointHistorySamples) {
        samples.shift();
    }
}

function recordMissingAccessPointSamples(groups) {
    const accessPoints = new Set(
        [...accessPointMetricHistory.keys()].map(key => key.slice(0, key.lastIndexOf(':')))
    );
    accessPoints.forEach(accessPoint => {
        if (groups.has(accessPoint)) {
            return;
        }
        appendMetricSample(accessPoint, 'latency', null);
        appendMetricSample(accessPoint, 'jitter', null);
        if (['latency', 'jitter'].every(metric =>
            (accessPointMetricHistory.get(`${accessPoint}:${metric}`) || []).every(value => value === null)
        )) {
            accessPointMetricHistory.delete(`${accessPoint}:latency`);
            accessPointMetricHistory.delete(`${accessPoint}:jitter`);
        }
    });
}

function createMetricSparkline(metric, samples) {
    const svgNamespace = 'http://www.w3.org/2000/svg';
    const width = 120;
    const height = 30;
    const padding = 3;
    const scaleMaximum = metric === 'latency' ? 200 : 50;
    const thresholds = metric === 'latency'
        ? [{ value: 50, level: 'good' }, { value: 100, level: 'medium' }]
        : [{ value: 10, level: 'good' }, { value: 30, level: 'medium' }];
    const plotHeight = height - padding * 2;
    const svg = document.createElementNS(svgNamespace, 'svg');
    svg.setAttribute('viewBox', `0 0 ${width} ${height}`);
    svg.setAttribute('role', 'img');
    svg.classList.add('metric-sparkline');
    svg.setAttribute('aria-label', `${metric === 'latency' ? 'Latenz' : 'Jitter'}-Verlauf der letzten bis zu 60 Wertänderungen`);

    const values = samples.filter(value => Number.isFinite(value));
    const quality = values.length ? getMetricQuality(metric, values[values.length - 1]) : { level: 'unavailable' };
    svg.classList.add(`sparkline-${quality.level}`);

    const title = document.createElementNS(svgNamespace, 'title');
    title.textContent = values.length
        ? `${values.length} unterschiedliche Messwerte; feste Skala 0–${scaleMaximum} ms. Grenzen: gut bis ${thresholds[0].value} ms, mittel bis ${thresholds[1].value} ms, darüber schlecht`
        : 'Noch keine Messpunkte';
    svg.appendChild(title);

    const baseline = document.createElementNS(svgNamespace, 'line');
    baseline.setAttribute('x1', padding);
    baseline.setAttribute('x2', width - padding);
    baseline.setAttribute('y1', height - padding);
    baseline.setAttribute('y2', height - padding);
    baseline.classList.add('sparkline-baseline');
    svg.appendChild(baseline);

    thresholds.forEach(threshold => {
        const marker = document.createElementNS(svgNamespace, 'line');
        const markerY = height - padding - threshold.value / scaleMaximum * plotHeight;
        marker.setAttribute('x1', padding);
        marker.setAttribute('x2', width - padding);
        marker.setAttribute('y1', markerY.toFixed(1));
        marker.setAttribute('y2', markerY.toFixed(1));
        marker.classList.add('sparkline-threshold', `sparkline-threshold-${threshold.level}`);
        svg.appendChild(marker);
    });

    if (values.length === 0) {
        return svg;
    }

    const points = samples.map((value, index) => {
        if (!Number.isFinite(value)) {
            return null;
        }
        const x = samples.length <= 1 ? width / 2 : padding + index * (width - 2 * padding) / (samples.length - 1);
        const boundedValue = Math.min(Math.max(value, 0), scaleMaximum);
        const y = height - padding - boundedValue / scaleMaximum * plotHeight;
        return `${x.toFixed(1)},${y.toFixed(1)}`;
    });

    let segment = [];
    const pathData = [];
    points.forEach(point => {
        if (point === null) {
            if (segment.length) {
                pathData.push(`M ${segment.join(' L ')}`);
                segment = [];
            }
        } else {
            segment.push(point);
        }
    });
    if (segment.length) {
        pathData.push(`M ${segment.join(' L ')}`);
    }

    const path = document.createElementNS(svgNamespace, 'path');
    path.setAttribute('d', pathData.join(' '));
    path.setAttribute('fill', 'none');
    path.setAttribute('stroke', 'currentColor');
    path.setAttribute('stroke-width', '2');
    path.setAttribute('stroke-linecap', 'round');
    path.setAttribute('stroke-linejoin', 'round');
    svg.appendChild(path);

    points.forEach(point => {
        if (point === null) {
            return;
        }
        const [cx, cy] = point.split(',');
        const marker = document.createElementNS(svgNamespace, 'circle');
        marker.setAttribute('cx', cx);
        marker.setAttribute('cy', cy);
        marker.setAttribute('r', '1.5');
        marker.setAttribute('fill', 'currentColor');
        svg.appendChild(marker);
    });
    return svg;
}

function updateDeviceAccessPointTable(devices) {
    const table = document.getElementById('deviceAccessPointTable');
    table.replaceChildren();

    const groups = new Map();
    devices.forEach(device => {
        if ((device.status || '').startsWith('Fehlt')) {
            return;
        }

        const accessPoint = device.connected_via || device.connectedVia || 'Unbekannt';
        if (!groups.has(accessPoint)) {
            groups.set(accessPoint, {
                icon: device.mesh_icon || '❓',
                deviceCount: 0,
                deviceNames: [],
                latencies: [],
                jitters: []
            });
        }

        const group = groups.get(accessPoint);
        group.deviceCount++;
        group.deviceNames.push(device.hostname || device.ip || 'Unbekanntes Gerät');
        const latency = Number(device.latency_ms);
        if (Number.isFinite(latency) && latency >= 0) {
            group.latencies.push(latency);
            const jitter = Number(device.jitter_ms);
            if (Number.isFinite(jitter) && jitter >= 0) {
                group.jitters.push(jitter);
            }
        }
    });

    if (groups.size === 0) {
        recordMissingAccessPointSamples(groups);
        const row = table.insertRow();
        const cell = row.insertCell();
        cell.colSpan = 4;
        cell.className = 'empty-state';
        cell.textContent = 'Keine online erkannten Geräte für eine Aggregation vorhanden.';
        return;
    }

    recordMissingAccessPointSamples(groups);

    [...groups.entries()]
        .sort(([left], [right]) => left.localeCompare(right))
        .forEach(([accessPoint, group]) => {
            const row = table.insertRow();
            row.insertCell().textContent = `${group.icon} ${accessPoint}`;
            const deviceCell = row.insertCell();
            const count = document.createElement('span');
            count.className = 'access-point-device-count';
            count.textContent = group.deviceCount;
            const names = document.createElement('small');
            names.className = 'access-point-device-names';
            names.textContent = group.deviceNames.join(', ');
            deviceCell.append(count, names);

            const averageLatency = group.latencies.length
                ? group.latencies.reduce((sum, value) => sum + value, 0) / group.latencies.length
                : null;
            const averageJitter = group.jitters.length
                ? group.jitters.reduce((sum, value) => sum + value, 0) / group.jitters.length
                : null;
            appendMetricSample(accessPoint, 'latency', averageLatency);
            appendMetricSample(accessPoint, 'jitter', averageJitter);

            const latencyMetric = document.createElement('div');
            latencyMetric.className = 'aggregated-metric';
            latencyMetric.appendChild(createQualityDisplay(
                averageLatency === null ? 'Keine Messung' : `${averageLatency.toFixed(1)} ms`,
                'latency',
                averageLatency !== null
            ));
            latencyMetric.appendChild(createMetricSparkline('latency', accessPointMetricHistory.get(`${accessPoint}:latency`) || []));
            row.insertCell().appendChild(latencyMetric);

            const jitterMetric = document.createElement('div');
            jitterMetric.className = 'aggregated-metric';
            jitterMetric.appendChild(createQualityDisplay(
                averageJitter === null ? 'Keine Messung' : `${averageJitter.toFixed(1)} ms`,
                'jitter',
                averageJitter !== null
            ));
            jitterMetric.appendChild(createMetricSparkline('jitter', accessPointMetricHistory.get(`${accessPoint}:jitter`) || []));
            row.insertCell().appendChild(jitterMetric);
        });
}

function updateInternetView(data, agents) {
    const serverLatency = Number(data.current_wan_latency);
    const serverJitter = Number(data.current_wan_jitter);
    const serverLatencyDisplay = formatMeasurementValue(serverLatency, 'latency', data.wan_last_measurement_at, data.wan_probe_successes);
    const serverJitterDisplay = formatMeasurementValue(serverJitter, 'jitter', data.wan_last_measurement_at, data.wan_probe_successes);
    setQualityDisplay(document.getElementById('serverLatency'), serverLatencyDisplay.text, 'latency', serverLatencyDisplay.available);
    setQualityDisplay(
        document.getElementById('serverJitter'),
        serverJitterDisplay.text,
        'jitter',
        serverJitterDisplay.available
    );
    document.getElementById('wanSuccessRate').textContent = formatProbeSuccesses(data.wan_probe_successes, data.wan_probe_attempts);
    document.getElementById('wanMeasurementAge').textContent = formatMeasurementAge(data.wan_last_measurement_at);
    document.getElementById('wanLastCheckAge').textContent = formatMeasurementAge(data.wan_last_checked_at);

    const agentEntries = Object.values(agents);
    const measuredAgents = agentEntries.filter(agent =>
        Number(agent.latency_ms) > 0 &&
        Number(agent.probe_successes) > 0 &&
        getMeasurementFreshness(agent.last_measurement_at).fresh
    );
    const averageLatency = measuredAgents.reduce((sum, agent) => sum + Number(agent.latency_ms), 0) / (measuredAgents.length || 1);
    const averageJitter = measuredAgents.reduce((sum, agent) => sum + (Number(agent.jitter_ms) || 0), 0) / (measuredAgents.length || 1);
    setQualityDisplay(
        document.getElementById('agentLatency'),
        measuredAgents.length ? `${averageLatency.toFixed(1)} ms` : 'Keine Messung',
        'latency',
        measuredAgents.length > 0
    );
    setQualityDisplay(
        document.getElementById('agentJitter'),
        measuredAgents.length ? `${averageJitter.toFixed(1)} ms` : 'Keine Messung',
        'jitter',
        measuredAgents.length > 0
    );
    document.getElementById('agentMeasurements').textContent = measuredAgents.length;
    document.getElementById('agentMeasurementsTotal').textContent = agentEntries.length;

    const table = document.getElementById('internetAgentTable');
    table.replaceChildren();
    if (agentEntries.length === 0) {
        const row = table.insertRow();
        const cell = row.insertCell();
        cell.colSpan = 9;
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
                agent.repeater_name || agent.access_point || 'LAN / Unbekannt'
            ].forEach(value => row.insertCell().textContent = value);
            const latencyMeasurement = formatMeasurementValue(agent.latency_ms, 'latency', agent.last_measurement_at, agent.probe_successes);
            row.insertCell().appendChild(createQualityDisplay(latencyMeasurement.text, 'latency', latencyMeasurement.available));
            const jitterMeasurement = formatMeasurementValue(agent.jitter_ms, 'jitter', agent.last_measurement_at, agent.probe_successes);
            row.insertCell().appendChild(createQualityDisplay(jitterMeasurement.text, 'jitter', jitterMeasurement.available));
            row.insertCell().textContent = formatMeasurementAge(agent.last_measurement_at);
            row.insertCell().textContent = formatMeasurementAge(agent.last_checked_at);
            row.insertCell().textContent = formatProbeSuccesses(agent.probe_successes, agent.probe_attempts);
            row.insertCell().textContent = agent.last_updated || '–';
        });
}

let callerUpdateInProgress = false;

function renderCallerDetails(element, fields) {
    element.replaceChildren();
    fields.forEach(([label, value, metric, available = true]) => {
        const term = document.createElement('dt');
        term.textContent = label;
        const description = document.createElement('dd');
        if (metric) {
            description.appendChild(createQualityDisplay(value, metric, available));
        } else {
            description.textContent = value || '–';
        }
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
                ['Latenz', formatMilliseconds(device.latency_ms), 'latency', Number(device.latency_ms) > 0],
                ['Jitter', Number.isFinite(Number(device.jitter_ms)) ? `${Number(device.jitter_ms).toFixed(1)} ms` : 'Keine Messung', 'jitter', Number(device.latency_ms) > 0 && Number.isFinite(Number(device.jitter_ms))]
            ]);
        }

        if (agent) {
            const latencyMeasurement = formatMeasurementValue(agent.latency_ms, 'latency', agent.last_measurement_at, agent.probe_successes);
            const jitterMeasurement = formatMeasurementValue(agent.jitter_ms, 'jitter', agent.last_measurement_at, agent.probe_successes);
            renderCallerDetails(document.getElementById('callerAgentDetails'), [
                ['Hostname', agent.hostname],
                ['Repeater', agent.repeater_name],
                ['Zugangspunkt', agent.access_point],
                ['Internet-Latenz', latencyMeasurement.text, 'latency', latencyMeasurement.available],
                ['Jitter', jitterMeasurement.text, 'jitter', jitterMeasurement.available],
                ['WLAN-Signal', Number(agent.wifi_rssi_dbm) < 0 ? `${agent.wifi_rssi_dbm} dBm` : 'Keine WLAN-Messung', 'wifi', Number(agent.wifi_rssi_dbm) < 0 && getMeasurementFreshness(agent.last_checked_at).fresh],
                ['Messalter', formatMeasurementAge(agent.last_measurement_at)],
                ['Letzte Prüfung', formatMeasurementAge(agent.last_checked_at)],
                ['Erfolgsquote', formatProbeSuccesses(agent.probe_successes, agent.probe_attempts)],
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
        const wanLatencyData = data.history.map(h =>
            h.wan_probe_successes > 0 ? h.wan_latency_ms : null
        );
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
            agentBody.replaceChildren();
            Object.values(agents).forEach(a => {
                const row = agentBody.insertRow();
                [
                    a.last_updated || '–',
                    a.ip || '–',
                    a.hostname || '–',
                    a.version ? `v${a.version}` : 'v1.0',
                    a.repeater_name || '🔌 LAN'
                ].forEach(value => row.insertCell().textContent = value);

                const latencyMeasurement = formatMeasurementValue(a.latency_ms, 'latency', a.last_measurement_at, a.probe_successes);
                row.insertCell().appendChild(createQualityDisplay(
                    latencyMeasurement.text,
                    'latency',
                    latencyMeasurement.available
                ));
                const jitterMeasurement = formatMeasurementValue(a.jitter_ms, 'jitter', a.last_measurement_at, a.probe_successes);
                row.insertCell().appendChild(createQualityDisplay(
                    jitterMeasurement.text,
                    'jitter',
                    jitterMeasurement.available
                ));
                const wifiFreshness = getMeasurementFreshness(a.last_checked_at);
                const wifiAvailable = Number(a.wifi_rssi_dbm) < 0 && wifiFreshness.fresh;
                row.insertCell().appendChild(createQualityDisplay(
                    Number(a.wifi_rssi_dbm) < 0
                        ? `${a.wifi_rssi_dbm} dBm${wifiAvailable ? '' : ' (veraltet)'}`
                        : '🔌 LAN',
                    'wifi',
                    wifiAvailable
                ));
                row.insertCell().textContent = formatMeasurementAge(a.last_measurement_at);
                row.insertCell().textContent = formatMeasurementAge(a.last_checked_at);
                row.insertCell().textContent = formatProbeSuccesses(a.probe_successes, a.probe_attempts);
                appendAgentMeshPath(agentBody, a, data.mesh_tree);
            });
        } else {
            agentBody.innerHTML = '<tr><td colspan="11" style="color: #888; font-style: italic;">Warte auf Agenten (Port 8081)...</td></tr>';
        }

        const tbody = document.getElementById('deviceTable');
        tbody.replaceChildren();

        devices.forEach(d => {
            const statusClass = d.status.startsWith('Fehlt') ? 'status-Fehlt' : 'status-Online';
            const hasMeasurement = !d.status.startsWith('Fehlt') && Number(d.latency_ms) >= 0;

            const connectedVia = d.connected_via || d.connectedVia || 'Unbekannt';
            const meshIcon = d.mesh_icon || '❓';
            const portValue = d.port || 'Unbekannt';

            const row = tbody.insertRow();
            const statusCell = row.insertCell();
            statusCell.className = statusClass;
            statusCell.textContent = d.status;

            const ipCell = row.insertCell();
            if ((portValue.includes('Web') || portValue.includes('UPnP')) && !d.status.startsWith('Fehlt')) {
                const scheme = portValue.includes('HTTPS') ? 'https://' : 'http://';
                let linkUrl = `${scheme}${d.ip}`;
                if (portValue.includes('Alt')) {
                    linkUrl = `${scheme}${d.ip}:8080`;
                }
                const link = document.createElement('a');
                link.href = linkUrl;
                link.target = '_blank';
                link.rel = 'noopener noreferrer';
                link.className = 'device-web-link';
                link.title = 'Webinterface öffnen';
                link.textContent = `🔗 ${d.ip}`;
                ipCell.appendChild(link);
            } else {
                ipCell.textContent = d.ip || '–';
            }

            const macCell = row.insertCell();
            macCell.className = 'mac-text';
            macCell.textContent = d.mac || '–';
            row.insertCell().textContent = d.hostname || '–';

            const meshCell = row.insertCell();
            meshCell.className = connectedVia === 'Unbekannt' ? 'mesh-unknown' : 'mesh-connected';
            meshCell.textContent = connectedVia === 'Unbekannt' ? '❓ Unbekannt' : `${meshIcon} ${connectedVia}`;
            row.insertCell().textContent = portValue;
            row.insertCell().appendChild(createQualityDisplay(
                hasMeasurement ? `${Number(d.latency_ms).toFixed(1)} ms` : 'Keine Messung',
                'latency',
                hasMeasurement
            ));
            const jitterAvailable = hasMeasurement && Number.isFinite(Number(d.jitter_ms));
            row.insertCell().appendChild(createQualityDisplay(
                jitterAvailable ? `${Number(d.jitter_ms).toFixed(1)} ms` : 'Keine Messung',
                'jitter',
                jitterAvailable
            ));
        });
        updateDeviceAccessPointTable(devices);

        if (!document.getElementById('caller').hidden) {
            updateCallerView();
        }
    } catch (err) {
        console.error("Fehler beim Abrufen der Daten:", err);
    }
}

setInterval(updateDashboard, 5000);
updateDashboard();
