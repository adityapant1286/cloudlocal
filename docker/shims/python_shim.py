import sys
import os
import json
import importlib

def run():
  # 1. Setup Path
  sys.path.append(os.getcwd())

  # 2. Parse Arguments
  try:
    handler_str = sys.argv[1]
    payload_str = sys.argv[2]

    module_name, function_name = handler_str.split('.')

    # 3. Dynamic Import
    if module_name in sys.modules:
      module = importlib.reload(sys.modules[module_name])
    else:
      module = importlib.import_module(module_name)

    handler = getattr(module, function_name)

    # 4. Parse Event
    event = json.loads(payload_str)
    context = {}  # Mock context object

    # 5. Execute Handler
    result = handler(event, context)

    # 6. Output ONLY the JSON result to stdout
    resp = json.dumps(result)
    sys.stdout.write(f"\n<CLOUDLOCAL_RESULT>{resp}</CLOUDLOCAL_RESULT>\n")
    sys.stdout.flush()

  except Exception as e:
    # Send the actual traceback/error to stderr for Go to capture
    sys.stderr.write(f"SHIM ERROR: {str(e)}")
    sys.exit(1)


if __name__ == "__main__":
  run()
