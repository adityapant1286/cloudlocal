const fs = require('fs');
const path = require('path');

async function run() {
  const [handlerPath, eventData] = process.argv.slice(2);
  const [fileName, functionName] = handlerPath.split('.');
  const event = JSON.parse(eventData);

  try {
    // Resolve the absolute path to the user's file
    const userModule = require(path.resolve(process.cwd(), `${fileName}.js`));
    const result = await userModule[functionName](event, {}); // {} is an empty context object
    process.stdout.write(JSON.stringify(result));
  } catch (err) {
    process.stderr.write(err.stack);
    process.exit(1);
  }
}

run();