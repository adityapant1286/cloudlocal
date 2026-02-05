import {
  styleSelectedElement,
  toISODateFormat
} from "./utils.js"

let lambdaCurrentFunction = null;

export async function lambdaLoadFunctions() {
  const res = await fetch('/dashboard/api/lambda/list');
  const data = await res.json();
  const container = document.getElementById('lambda-function-list');

  if (data && data.Functions) {
    container.innerHTML = data.Functions.map(f => `
        <div class="flex items-center group px-2 rounded-lg hover:bg-neutral-800 transition-all cls-lambda-function-parent">
            <button onclick="lambdaSelectFunction(this)" data-payload='${JSON.stringify(f)}'
             class="cls-btn-lambda-func flex-1 text-left py-3 text-sm flex items-center gap-2 overflow-hidden">
                <svg class="w-6 h-6 text-blue-500 shrink-0" fill="currentColor" viewBox="0 0 20 20"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg>
                <span class="text-slate-300 truncate">${f.FunctionName}</span>
            </button>
            
            <button onclick="lambdaDeleteFunction('${f.FunctionName}')" class="p-2 text-gray-400 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-opacity">
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
            </button>
        </div>
    `).join('');
  }
}

export async function lambdaSelectFunction(e, functionConfigs) {
  const lambdaFunction = functionConfigs ? functionConfigs : JSON.parse(e.dataset.payload);
  lambdaCurrentFunction = lambdaFunction;
  const lambdaFunctionCodeSection = document.getElementById('lambda-function-code-section');

  if (e) {
    styleSelectedElement(e.parentElement, 'div.cls-lambda-function-parent');
  }
  const lambdaFunctionGrid = document.getElementById('lambda-active-function-panel');
  lambdaFunctionGrid.innerHTML = "";
  lambdaFunctionGrid.classList.add('hidden');
  lambdaFunctionCodeSection.classList.add('hidden');

  document.getElementById('lambda-workspace').classList.remove('hidden');
  document.getElementById('lambda-active-function-name').innerText = lambdaFunction.FunctionName;

  lambdaFunctionGrid.innerHTML = Object.entries(lambdaFunction)
  .map(([k, v]) => `
      <div class="p-2">
        <div class="text-xs font-bold text-gray-400 tracking-tighter">${k}</div>
        <div class="text-xs text-white mt-2">${k === "LastModified" ? toISODateFormat(v) : v}</div>
      </div>
  `).join('');
  lambdaFunctionGrid.classList.remove('hidden');

  const codeContent = await lambdaRetrieveSourceCode(lambdaFunction.FunctionName);

  if (codeContent) {
    lambdaFunctionCodeSection.classList.remove('hidden');
    document.getElementById('lambda-function-code-input').value = codeContent.sourceCode;
  }

}

export async function lambdaRetrieveSourceCode(functionName) {
  const res = await fetch(
      `/dashboard/api/lambda/retrieve-code?name=${encodeURIComponent(functionName)}`,
      {method: 'POST'}
  );
  return await res.json();
}

export async function lambdaDeleteFunction(functionName) {

  if (functionName === undefined) {
    functionName = lambdaCurrentFunction.FunctionName;
  }

  if (!confirm(`Delete function "${functionName}"?`)) {
    return;
  }

  const res = await fetch(
      `/dashboard/api/lambda/delete?name=${encodeURIComponent(functionName)}`,
      {method: 'POST'}
  );
  if (res.ok) {
    if (lambdaCurrentFunction.FunctionName === functionName) {
      document.getElementById('lambda-workspace').classList.add('hidden');
      lambdaCurrentFunction = null;
    }
    await lambdaLoadFunctions();
  } else {
    const err = await res.json();
    alert(`Error: ${err.Message || "Functions might not be exists"}`);
  }

}

export function lambdaOpenCreateFunctionModal() {
  document.getElementById('lambda-create-modal').classList.remove('hidden');
  document.getElementById('lambda-function-name').value = "";
  document.getElementById('lambda-runtime-selector').selectedIndex = 0;
  document.getElementById('lambda-function-handler').value = "";
  document.getElementById('lambda-code-upload').value = "";
}

export function lambdaCloseCreateFunctionModal() {
  document.getElementById('lambda-create-modal').classList.add('hidden');
}

export function onLambdaFunctionNameInput(e) {
  const functionName = e.value;

  const regex = /^[a-zA-Z][a-zA-Z0-9_-]*?$/;
  const isValid = regex.test(functionName) && !functionName.includes('..');
  const btn = document.getElementById('lambda-btn-confirm-create');
  const hint = document.getElementById('lambda-function-name-hint');

  btn.disabled = !isValid;
  hint.className = isValid
      ? "mt-2 text-[10px] text-emerald-500"
      : "mt-2 text-[10px] text-red-500";

  document.getElementById('lambda-function-handler').value =
      isValid && functionName
          ? functionName + "."
          : "";
}

export async function lambdaPerformCreateFunction() {
  const functionName = document.getElementById('lambda-function-name').value;
  const functionRuntime = document.getElementById('lambda-runtime-selector').value;
  const functionHandler = document.getElementById('lambda-function-handler').value;
  const eleFunctionCode = document.getElementById('lambda-code-upload');
  const eleFunctionCodeStatus = document.getElementById('lambda-function-code-hint');

  if (eleFunctionCode.files.length === 0) {
    eleFunctionCodeStatus.className = "mt-2 text-[10px] text-red-500";
    eleFunctionCodeStatus.value = "Please select a file first.";
    return;
  }
  eleFunctionCodeStatus.value = "";
  eleFunctionCodeStatus.className = "mt-2 text-[10px] text-neutral-500 italic";

  const codeFile = eleFunctionCode.files[0];
  const reader = new FileReader();

  reader.onload = async (event) => {
    const base64String = event.target.result.split(',')[1];
    // const arrayBuffer = event.target.result;
    try {
      eleFunctionCodeStatus.innerText = "Processing...";
      const res = await fetch(
          '/dashboard/api/lambda/create-function',
          {
            method: 'POST',
            body: JSON.stringify({
              FunctionName: functionName,
              Runtime: functionRuntime,
              Role: "arn:aws:iam::123456789012:role/service-role/lambda-role",
              Handler: functionHandler,
              Code: {
                ZipFile: base64String
              }
            })
      });

      if (!res.ok) {
        eleFunctionCodeStatus.value = "Code upload failed." + res.statusText;
      } else {
        eleFunctionCodeStatus.value = "Lambda function created successfully!";

        await lambdaLoadFunctions();

        lambdaCloseCreateFunctionModal();
      }
    } catch (error) {
      eleFunctionCodeStatus.value = "Error: " + error.message;
    }
  };

  reader.readAsDataURL(codeFile);
}

export async function lambdaFunctionInvoke(e) {
  const lambdaPayload = document.getElementById('lambda-function-payload-input').value;
  const res = await fetch(
      '/dashboard/api/lambda/invoke-function',
      {
        method: 'POST',
        body: JSON.stringify({
          FunctionName: lambdaCurrentFunction.FunctionName,
          payload: lambdaPayload
        })
  });
  let resp = await res.json();
  document.getElementById('lambda-function-output').innerText = JSON.stringify(resp);
}

export function lambdaOpenUpdateFunctionModal() {
  document.getElementById('lambda-update-code-modal').classList.remove('hidden');
  document.getElementById('lambda-function-update-name').innerText = lambdaCurrentFunction.FunctionName;
  document.getElementById('lambda-function-update-runtime').innerText = lambdaCurrentFunction.Runtime;
  document.getElementById('lambda-function-update-handler').value = lambdaCurrentFunction.Handler;
  document.getElementById('lambda-update-code-upload').value = "";
}

export function lambdaCloseUpdateFunctionModal() {
  document.getElementById('lambda-update-code-modal').classList.add('hidden');
}

export async function lambdaPerformUpdateFunction(){

  const functionHandler = document.getElementById('lambda-function-update-handler').value;
  const eleUpdatedCode = document.getElementById('lambda-update-code-upload');
  const eleUpdatedCodeStatus = document.getElementById('lambda-function-update-code-hint');

  if (eleUpdatedCode.files.length === 0) {
    eleUpdatedCodeStatus.className = "mt-2 text-[10px] text-red-500";
    eleUpdatedCodeStatus.value = "Please select a file first.";
    return;
  }
  eleUpdatedCodeStatus.value = "";
  eleUpdatedCodeStatus.className = "mt-2 text-[10px] text-neutral-500 italic";

  const codeFile = eleUpdatedCode.files[0];
  const reader = new FileReader();

  reader.onload = async (event) => {
    const base64String = event.target.result.split(',')[1];
    // const arrayBuffer = event.target.result;
    try {
      eleUpdatedCodeStatus.innerText = "Processing...";
      const res = await fetch(
          '/dashboard/api/lambda/update-function',
          {
            method: 'POST',
            body: JSON.stringify({
              FunctionName: lambdaCurrentFunction.FunctionName,
              Handler: functionHandler,
              Code: {
                ZipFile: base64String
              }
            })
          });

      if (!res.ok) {
        eleUpdatedCodeStatus.value = "Code upload failed." + res.statusText;
      } else {
        eleUpdatedCodeStatus.value = "Lambda function updated successfully!";
        const data = await res.json();

        await lambdaSelectFunction(undefined, data.Function);

        lambdaCloseUpdateFunctionModal();
      }
    } catch (error) {
      eleUpdatedCodeStatus.value = "Error: " + error.message;
    }
  };

  reader.readAsDataURL(codeFile);
}