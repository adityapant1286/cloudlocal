import {
  styleSelectedElement,
  openCellModal
} from "./utils.js"

// S3 Manager
let s3CurrentBucket = "";
let s3CurrentBucketPath = "";

export async function s3LoadBuckets() {
  const res = await fetch('/dashboard/api/s3/list-buckets');
  const data = await res.json();
  const container = document.getElementById('s3-bucket-list');

  if (data && data.Buckets) {
    container.innerHTML = data.Buckets.map(b => `
        <div class="flex items-center group px-2 rounded-lg hover:bg-neutral-800 transition-all cls-s3-bucket-parent">
            <button onclick="selectS3Bucket(this, '${b.Name}', '${b.Path}')" class="cls-s3-bucket flex-1 text-left py-3 text-sm flex items-center gap-2 overflow-hidden">
                <svg class="w-6 h-6 text-blue-500 shrink-0" fill="currentColor" viewBox="0 0 20 20"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg>
                <span class="text-slate-300 truncate">${b.Name}</span>
            </button>
            
            <button onclick="s3DeleteBucket('${b.Name}')" class="p-2 text-gray-400 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-opacity">
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
            </button>
        </div>
    `).join('');
  }
}

export function s3ValidateBucketName(name) {
  const regex = /^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$/;
  const isValid = regex.test(name) && !name.includes('..');
  const btn = document.getElementById('s3-btn-confirm-create');
  const hint = document.getElementById('s3-bucket-hint');

  btn.disabled = !isValid;
  hint.className = isValid
      ? "mt-2 text-[10px] text-emerald-500"
      : "mt-2 text-[10px] text-red-500";
}

export function s3OpenCreateBucketModal() {
  document.getElementById('s3-create-bucket-modal').classList.remove('hidden');
  document.getElementById('s3-new-bucket-name').value = "";
  s3ValidateBucketName("");
}

export function s3CloseCreateBucketModal() {
  document.getElementById('s3-create-bucket-modal').classList.add('hidden');
}

export async function s3PerformCreateBucket() {
  const name = document.getElementById('s3-new-bucket-name').value;
  const res = await fetch(
      `/dashboard/api/s3/create-bucket?name=${name}`,
      {method: 'POST'}
  );
  if (res.ok) {
    s3CloseCreateBucketModal();
    await s3LoadBuckets();
  }
}

export async function s3EmptyBucket() {
  if (!confirm(
      `Are you sure you want to DELETE ALL objects in "${s3CurrentBucket}"?`)) {
    return;
  }

  // 1. Fetch all objects
  const res = await fetch(`/dashboard/api/s3/list-objects?bucket=${s3CurrentBucket}`);
  const data = await res.json();

  if (!data.Contents || data.Contents.length === 0) {
    alert("Bucket is already empty.");
    return;
  }

  // 2. Delete each object
  // In a production app, you'd use DeleteObjects (plural) API,
  // but for local dev, iterating is fine and shows progress.
  for (const obj of data.Contents) {
    await fetch(
        `/dashboard/api/s3/delete?bucket=${s3CurrentBucket}&key=${encodeURIComponent(obj.Key)}`,
        {method: 'POST'}
    );
  }

  alert(`Emptied ${data.Contents.length} objects.`);
  await selectS3Bucket(undefined, s3CurrentBucket, s3CurrentBucketPath); // Refresh view
}

export async function s3DeleteBucket(name) {
  if (!confirm(`Delete bucket "${name}"? Note: Bucket must be empty first.`)) {
    return;
  }

  const res = await fetch(
      `/dashboard/api/s3/delete-bucket?name=${encodeURIComponent(name)}`,
      {method: 'POST'}
  );
  if (res.ok) {
    if (s3CurrentBucket === name) {
      document.getElementById('s3-workspace').classList.add('hidden');
      s3CurrentBucket = "";
    }
    await s3LoadBuckets();
  } else {
    const err = await res.json();
    alert(`Error: ${err.Message || "Bucket might not be empty"}`);
  }
}

export async function selectS3Bucket(e, name, path) {
  s3CurrentBucket = name;
  s3CurrentBucketPath = path;
  document.getElementById('s3-active-bucket-name').innerText = name;
  document.getElementById('s3-active-bucket-path').innerText = path;
  document.getElementById('s3-workspace').classList.remove('hidden');

  if (e) {
    styleSelectedElement(e.parentElement, 'div.cls-s3-bucket-parent');
  }
  const res = await fetch(`/dashboard/api/s3/list-objects?bucket=${name}`);
  const data = await res.json();
  const container = document.getElementById('s3-object-list');

  if (!data.Contents) {
    container.innerHTML = '<tr><td colspan="3" class="p-10 text-center text-gray-600">No objects found</td></tr>';
    return;
  }

  container.innerHTML = data.Contents.map(obj => {
    const safeKey = encodeURIComponent(obj.Key);
    return `
        <tr class="hover:bg-neutral-800/30 group border-b border-neutral-800/50 transition-colors">
        <td class="p-3">
            <button onclick="s3ViewObject('${safeKey}')" 
                    class="text-blue-400 hover:text-blue-300 hover:underline text-left transition-colors">
                ${obj.Key}
            </button>
        </td>
        <td class="p-3 text-gray-400 font-mono">${(obj.Size / 1024).toFixed(2)} KB</td>
        <td class="p-3 text-right">
            <button onclick="s3DeleteObject('${safeKey}')" class="text-red-500 hover:text-red-400 p-1">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                </svg>
            </button>
        </td>
    </tr>
    `;
  }).join('');
}

export function s3OpenUploadModal() {
  document.getElementById('s3-upload-bucket-name').innerText = s3CurrentBucket;
  document.getElementById('s3-upload-modal').classList.remove('hidden');
  document.getElementById('s3-upload-key').value = "";
  document.getElementById('s3-upload-content').value = "";
}

export function s3CloseUploadModal() {
  document.getElementById('s3-upload-modal').classList.add('hidden');
}

export async function s3PerformUpload() {
  const key = document.getElementById('s3-upload-key').value;
  const content = document.getElementById('s3-upload-content').value;

  if (!key) {
    return alert("Object key is required");
  }

  try {
    const res = await fetch(
        '/dashboard/api/s3/upload',
        {
          method: 'POST',
          body: JSON.stringify({
            Bucket: s3CurrentBucket,
            Key: key,
            Body: btoa(content) // Encode to base64 for safe transit
          })
    });

    if (!res.ok) {
      throw new Error("Upload failed");
    }

    s3CloseUploadModal();

    await selectS3Bucket(undefined, s3CurrentBucket, s3CurrentBucketPath); // Refresh the list

  } catch (err) {
    alert(err.message);
  }
}

export async function s3DeleteObject(key) {
  if (!confirm(`Delete ${key} from ${s3CurrentBucket}?`)) {
    return;
  }

  try {
    const res = await fetch(
        `/dashboard/api/s3/delete?bucket=${s3CurrentBucket}&key=${encodeURIComponent(key)}`,
        {method: 'POST'}
    );

    if (!res.ok) {
      throw new Error("Delete failed");
    }

    await selectS3Bucket(undefined, s3CurrentBucket, s3CurrentBucketPath); // Refresh the list

  } catch (err) {
    alert(err.message);
  }
}

export async function s3ViewObject(key) {
  // In a real AWS environment, you'd use a Presigned URL.
  // Locally, we can just fetch the object data.
  const res = await fetch(`/dashboard/api/s3/get-object?bucket=${s3CurrentBucket}&key=${encodeURIComponent(key)}`);
  const data = await res.json();

  // Decode from base64 (AWS returns Body as base64 in many JSON proxies)
  const content = atob(data.Body);

  openCellModal(`S3: ${key}`, content);
}