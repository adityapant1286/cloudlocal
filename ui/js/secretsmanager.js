import {
  styleSelectedElement
} from "./utils.js"

// Secrets manager
let smCurrentSecret = null;
let smCurrentSecretValue = "";
let secretModalMode = 'create'

export async function loadSecrets() {
  const res = await fetch('/dashboard/api/secrets/list');
  const data = await res.json();
  const container = document.getElementById('sm-secrets-list');

  container.innerHTML = data.SecretList.map(s => `
        <button onclick="selectSecret(this, '${s.Name}')" class="cls-sm-secret w-full text-left px-4 py-3 rounded-lg text-sm transition-all hover:bg-neutral-800 group border border-transparent hover:border-neutral-700">
            <div class="text-slate-300 font-medium">${s.Name}</div>
            <div class="text-[10px] text-gray-300 truncate">${s.ARN}</div>
        </button>
    `).join('');
}

export async function selectSecret(e, name) {
  const res = await fetch(`/dashboard/api/secrets/get?name=${name}`);
  const data = await res.json();

  smCurrentSecret = name;
  smCurrentSecretValue = data.SecretString;

  styleSelectedElement(e, 'button.cls-sm-secret');

  document.getElementById('sm-secret-details').classList.remove('hidden');
  document.getElementById('sm-active-secret-name').innerText = name;

  document.getElementById('sm-active-secret-arn').innerText = data.ARN;

  const display = document.getElementById('sm-secret-value-display');
  display.innerText = smCurrentSecretValue;
  display.classList.add('blur-sm'); // Always start blurred
  document.getElementById('sm-visibility-btn').innerText = "Reveal";
}

export function toggleSecretVisibility() {
  const display = document.getElementById('sm-secret-value-display');
  const btn = document.getElementById('sm-visibility-btn');

  if (display.classList.contains('blur-sm')) {
    display.classList.remove('blur-sm', 'select-none');
    btn.innerText = "Hide";
  } else {
    display.classList.add('blur-sm', 'select-none');
    btn.innerText = "Reveal";
  }
}

export function openSecretModal(mode) {
  secretModalMode = mode;
  const modal = document.getElementById('secret-modal');
  const nameInput = document.getElementById('sm-modal-secret-name');
  const valueInput = document.getElementById('sm-modal-secret-value');
  const title = document.getElementById('sm-secret-modal-title');
  const nameGroup = document.getElementById('sm-secret-name-group');

  modal.classList.remove('hidden');
  document.getElementById('sm-secret-modal-error').classList.add('hidden');

  if (mode === 'edit') {
    title.innerText = "Edit Secret Value";
    nameGroup.classList.add('hidden'); // Cannot rename via PutSecretValue
    valueInput.value = smCurrentSecretValue;
  } else {
    title.innerText = "Create New Secret";
    nameGroup.classList.remove('hidden');
    nameInput.value = "";
    valueInput.value = "";
  }
}

export function closeSecretModal() {
  document.getElementById('secret-modal').classList.add('hidden');
}

export function formatSecretJson() {
  const input = document.getElementById('sm-modal-secret-value');
  try {
    const obj = JSON.parse(input.value);
    input.value = JSON.stringify(obj, null, 2);
  } catch (e) {
    alert("Not a valid JSON string");
  }
}

export async function saveSecret() {
  const name = document.getElementById('sm-modal-secret-name').value;
  const value = document.getElementById('sm-modal-secret-value').value;
  const errorEl = document.getElementById('sm-secret-modal-error');

  // For 'create', we use a specific API. For 'edit', we use another.
  const endpoint = secretModalMode === 'create'
      ? '/dashboard/api/secrets/create'
      : '/dashboard/api/secrets/put';

  const payload = secretModalMode === 'create'
      ? {Name: name, SecretString: value}
      : {SecretId: smCurrentSecret, SecretString: value};

  try {
    const res = await fetch(endpoint, {
      method: 'POST',
      body: JSON.stringify(payload)
    });

    if (!res.ok) {
      throw new Error(await res.text());
    }

    closeSecretModal();
    await loadSecrets(); // Refresh sidebar
    if (secretModalMode === 'edit') {
      await selectSecret(smCurrentSecret); // Refresh view
    }
  } catch (err) {
    errorEl.innerText = err.message;
    errorEl.classList.remove('hidden');
  }
}

export async function deleteSecret() {
  if (!confirm(`Are you sure you want to delete ${smCurrentSecret}?`)) {
    return;
  }

  try {
    const res = await fetch(
        `/dashboard/api/secrets/delete?name=${smCurrentSecret}`, {
          method: 'POST'
        });

    if (!res.ok) {
      throw new Error("Failed to delete secret");
    }

    document.getElementById('sm-secret-details').classList.add('hidden');
    smCurrentSecret = null;
    await loadSecrets();
  } catch (err) {
    alert(err.message);
  }
}