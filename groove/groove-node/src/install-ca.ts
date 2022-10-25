#!/usr/bin/env node

import { Groove } from './index';
import { exec } from 'child_process';
import { promisify } from 'util';

const main = async () => {
    const proxy = new Groove({});
    const executable = await proxy.getExecutablePath();

    try {
        await promisify(exec)(`${executable} install-ca`);
    } catch (error: any) {        
        console.log(`Error while installing CA: ${error.stderr}`)
        process.exit()
    }
    console.log("CA Install Success")
}

main();
