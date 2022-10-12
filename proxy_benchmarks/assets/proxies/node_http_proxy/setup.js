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

    const { certsFolder } = certificate;

    console.log("Will add root certificate to keychain...")

    // Trust this newly generated file
    execSync(`sudo security add-trusted-cert -d -p ssl -p basic -k /Library/Keychains/System.keychain ${certsFolder}/ca.pem`);

    console.log(chalk.green("Proxy setup succeeded."))
}

main();
