import * as DDB from './js/dynamodb.js';
import * as SM from './js/secretsmanager.js';
import * as KMS from './js/kms.js';
import * as SQS from './js/sqs.js';
import * as S3 from './js/s3.js';
import * as LAMBDA from './js/lambda.js';
import * as CW from './js/cloudwatch.js';
import * as UTILS from './js/utils.js';
import * as OVERVIEW from "./js/overview.js";


const modules = [DDB, SM, KMS, SQS, S3, LAMBDA, CW, UTILS, OVERVIEW];

modules.forEach(mod => {
  Object.keys(mod).forEach(key => {
    window[key] = mod[key];
  });
});

export async function showView(viewId) {
  // Hide all views
  document.querySelectorAll('.view-container').forEach(el => el.classList.add('hidden'));

  document.getElementById(`view-${viewId}`).classList.remove('hidden');

  try {
    switch (viewId) {
      case "overview":
        await OVERVIEW.fetchOverviewData();
        return
      case 'dynamodb':
        await DDB.ddbLoadTables();
        return;
      case 'secrets':
        await SM.loadSecrets();
        return;
      case 'kms':
        await KMS.loadKmsKeys();
        return;
      case 'sqs':
        await SQS.sqsLoadQueues();
        return;
      case 's3':
        await S3.s3LoadBuckets();
        return;
      case 'lambda':
        await LAMBDA.lambdaLoadFunctions();
        return;
      case 'cloudwatch':
        await CW.cwLoadLogGroups()
        return;
    }
  } catch (e) {
    console.error("Failed to load " + viewId)
    console.error(e);
  }
}
window.showView = showView; // Make sure this is global too

// 4. Initialization
window.addEventListener('DOMContentLoaded', () => {
  window.switchLog(); // Initializing logs
  showView('overview');
});