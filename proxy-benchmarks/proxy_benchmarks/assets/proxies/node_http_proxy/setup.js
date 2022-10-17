const ca = require('@bjowes/http-mitm-proxy/lib/ca.js');
const path = require('path');
const chalk = require('chalk');
const { execSync } = require('child_process');

const installCertificate = (certificateDirectory) => {
    return new Promise((resolve, reject) => {
        ca.create(certificateDirectory, function (err, ca) {
            if (err) reject(err);
            else resolve(ca);
        });
    });
}

const main = async () => {
    // https://github.com/bjowes/node-http-mitm-proxy/blob/master/lib/proxy.js#L59
    const defaultCertificateDirectory = path.resolve(process.cwd(), '.http-mitm-proxy');

    let certificate = null;

    try {
        certificate = await installCertificate(defaultCertificateDirectory);
    } catch (e) {
        console.log(chalk.red(`Error: ${e}`));
        process.exit(1);
    }

    console.log(chalk.green("Certificate generation succeeded."))
}

main();
