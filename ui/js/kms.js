import {
  styleSelectedElement
} from "./utils.js"

// KMS manager
let currentKmsKey = null;

export async function loadKmsKeys() {
  const res = await fetch('/dashboard/api/kms/list');
  const data = await res.json();
  const container = document.getElementById('kms-key-list');

  if (data && data.Keys) {
    container.innerHTML = data.Keys.map(k => `
        <button onclick="selectKmsKey(this, '${k.KeyId}', '${k.Arn}')" class="cls-btn-kms-key w-full text-left px-4 py-3 rounded-lg text-sm transition-all hover:bg-neutral-800 focus:bg-neutral-800 group border border-transparent hover:border-neutral-700">
            <div class="text-slate-300 font-mono text-[11px] truncate">${k.KeyId}</div>
            <div class="text-slate-400 font-mono text-[10px] truncate">${k.Description}</div>
        </button>
    `).join('');
  }
}

export async function loadKmsAliases(kmsKeyId) {
  const aliasContainer = document.getElementById('kms-active-key-aliases');
  aliasContainer.innerHTML = '';
  document.getElementById('kms-active-key-alias-panel').classList.add('hidden');
  // key aliases
  const res = await fetch(
      `/dashboard/api/kms/list-aliases?keyId=${kmsKeyId}`,
      {method: 'POST'}
  );
  const data = await res.json();
  if (data && data.Aliases) {
    aliasContainer.innerHTML = data.Aliases.map(a => `
      <div class="text-slate-50 font-mono text-[11px] truncate bg-slate-700 w-fit rounded-full py-1 px-2">${a.AliasName}</div>
    `).join('');
    document.getElementById('kms-active-key-alias-panel').classList.remove('hidden');
  }

}

export async function selectKmsKey(e, kmsKeyId, kmsKeyArn) {
  currentKmsKey = {
    KeyId: kmsKeyId,
    Arn: kmsKeyArn,
  };

  styleSelectedElement(e, 'button.cls-btn-kms-key');

  document.getElementById('kms-workspace').classList.remove('hidden');
  document.getElementById('kms-active-key-id').innerText = kmsKeyId;
  document.getElementById('kms-active-key-arn').innerText = kmsKeyArn;

  await loadKmsAliases(kmsKeyId);

}

export async function kmsEncrypt() {
  const text = document.getElementById('kms-encrypt-input').value;
  const res = await fetch(
      '/dashboard/api/kms/encrypt',
      {
        method: 'POST',
        body: JSON.stringify({
          KeyId: currentKmsKey.KeyId,
          Plaintext: btoa(text) // AWS KMS expects base64 plaintext
        })
  });
  const data = await res.json();
  document.getElementById('kms-encrypt-output').innerText = data.CiphertextBlob;
}

export async function kmsDecrypt() {
  const blob = document.getElementById('kms-decrypt-input').value;
  const res = await fetch(
      '/dashboard/api/kms/decrypt',
      {
        method: 'POST',
        body: JSON.stringify({CiphertextBlob: blob})
      }
  );
  const data = await res.json();
  // Decode base64 result back to text
  document.getElementById('kms-decrypt-output').innerText = atob(data.Plaintext);
}

export async function createKmsKey() {
  await fetch('/dashboard/api/kms/create', {method: 'POST'});
  await loadKmsKeys();
}

export function openKmsCreateAliasModel() {
  document.getElementById('kms-create-alias-modal').classList.remove('hidden');
  document.getElementById('kms-create-alias-key-id').innerHTML = currentKmsKey.KeyId;
}

export function kmsCloseCreateAliasModal() {
  document.getElementById('kms-create-alias-modal').classList.add('hidden');
}

export async function kmsPerformCreateKmsKeyAlias() {
  let name = document.getElementById('kms-new-alias-name').value;

  if (!name.includes("/") && !name.startsWith("alias/")) {
    name = "alias/" + name;
  }

  const res = await fetch(
      `/dashboard/api/kms/create-alias?keyId=${encodeURIComponent(currentKmsKey.KeyId)}&name=${encodeURIComponent(name)}`,
      {method: 'POST'}
  );
  if (res.ok) {
    kmsCloseCreateAliasModal();
    await loadKmsAliases(currentKmsKey.KeyId);
  } else {
    const data = await res.json();
    document.getElementById('kms-create-alias-hint').innerText = data.message;
  }
}