const viewFiles = [
    'overview',
    'agents',
    'devices',
    'mesh',
    'internet',
    'caller'
];

async function loadFrontend() {
    const views = await Promise.all(viewFiles.map(async name => {
        const response = await fetch(`/static/views/${name}.html`);
        if (!response.ok) {
            throw new Error(`Ansicht "${name}" konnte nicht geladen werden (HTTP ${response.status})`);
        }
        return response.text();
    }));

    const glossaryResponse = await fetch('/static/views/glossary.html');
    if (!glossaryResponse.ok) {
        throw new Error(`Begriffserklärungen konnten nicht geladen werden (HTTP ${glossaryResponse.status})`);
    }

    document.getElementById('pageViews').innerHTML = views.join('\n');
    document.getElementById('pageGlossary').innerHTML = await glossaryResponse.text();

    const script = document.createElement('script');
    script.src = '/static/script.js';
    script.onerror = () => {
        showLoadError('Dashboard-Skripte konnten nicht geladen werden.');
    };
    document.body.appendChild(script);
}

function showLoadError(message) {
    const pageViews = document.getElementById('pageViews');
    pageViews.replaceChildren();
    const error = document.createElement('p');
    error.className = 'load-error';
    error.setAttribute('role', 'alert');
    error.textContent = message;
    pageViews.appendChild(error);
}

loadFrontend().catch(error => {
    console.error('Frontend konnte nicht vollständig geladen werden:', error);
    showLoadError('Die Dashboard-Ansichten konnten nicht geladen werden. Bitte Seite neu laden.');
});
