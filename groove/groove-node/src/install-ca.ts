#!/usr/bin/env node

import { getExecutable } from './index';
import { exec } from 'child_process';
import { promisify } from 'util';

const main = async () => {
    const executable = await getExecutable();

    try {
        await promisify(exec)(`${executable} install-ca`);
    } catch (error: any) {        
        console.log(`Error while installing CA: ${error.stderr}`)
        process.exit()
    }
    console.log("CA Install Success")
}

main();
