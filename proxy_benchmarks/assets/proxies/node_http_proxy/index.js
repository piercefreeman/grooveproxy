const Proxy = require('@bjowes/http-mitm-proxy');
const { program } = require('commander');
const chalk = require('chalk');

// Allow self-signed certificates during fetch, since the local harness launches
// a web service that generates its own self-signed certificates.
process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

program
  .option('--port <int>')

program.parse();
const { port } = program.opts();

var proxy = Proxy();

proxy.onError(function(ctx, err) {
  const url = ctx && ctx.clientToProxyRequest ? ctx.clientToProxyRequest.url : "";
  console.error(`Proxy error on ${url}:`, err);
  if (err.code === "ERR_SSL_SSLV3_ALERT_CERTIFICATE_UNKNOWN") {
      console.log(chalk.red("SSL certification failed.\nIt's likely you haven't installed the root certificate on your machine."));

      // This will add a `NodeMITMProxyCA` cert to your local desktop keychain
      console.log(chalk.red("MacOS: security add-trusted-cert -r trustRoot -k ~/Library/Keychains/login.keychain-db ./.http-mitm-proxy/certs/ca.pem"));
  }
});


const exitOnSignal = (signal) => {
  process.on(signal, () => {
    console.log('\nCaught ' + signal + ', exiting');
    proxy.close();
    process.exit(1);
  });
}

exitOnSignal('SIGINT');
exitOnSignal('SIGTERM');

console.log(`Will launch proxy on port ${port}`)

proxy.listen({port: port});
