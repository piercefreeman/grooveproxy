import { promisify } from 'util';
import { exec, spawn } from 'child_process';
import { stat } from 'fs/promises'
import { join } from 'path';

interface GrooveConfiguration {
    commandTimeout?: number;
    port?: number;
    controlPort?: number;
    proxyServer?: string;
    proxyUsername?: string;
    proxyPassword?: string;
    authUsername?: string;
    authPassword?: string;
}

class Groove {
    process: any
    executablePath: string | null

    commandTimeout: number
    port: number
    controlPort: number
    proxyServer: string | null
    proxyUsername: string | null
    proxyPassword: string | null
    authUsername: string | null
    authPassword: string | null

    constructor(config: GrooveConfiguration) {
        this.process = null;
        this.executablePath = null;

        this.commandTimeout = config.commandTimeout || 5;
        this.port = config.port || 6010;
        this.controlPort = config.controlPort || 6011;
        this.proxyServer = config.proxyServer || null;
        this.proxyUsername = config.proxyUsername || null;
        this.proxyPassword = config.proxyPassword || null;
        this.authUsername = config.authUsername || null;
        this.authPassword = config.authPassword || null;
    }

    async launch() {
        if (this.process != null) {
            throw Error("Already spawned proxy.")
        }

        const parameters = {
            "--port": this.port ? this.port.toString() : null,
            "--control-port": this.controlPort ? this.controlPort.toString() : null,
            "--proxy-server": this.proxyServer,
            "--proxy-username": this.proxyUsername,
            "--proxy-password": this.proxyPassword,
            "--auth-username": this.authUsername,
            "--auth-password": this.authPassword,
        }

        const exc = await this.getExecutablePath()
        this.process = spawn(
            exc,
            Object.entries(parameters).reduce((previous: string[], [key, value] : [string, string | null]) => {    
                if (value != null) {
                    previous = [...previous, key, value]
                }
                return previous;
            }, [])
        );

        this.process.stdout.setEncoding('utf8');
        this.process.stdout.on('data', (data: string) => {
            console.log(`proxy-stdout: ${data}`);
        });

        this.process.stderr.setEncoding('utf8');
        this.process.stderr.on('data', (data: string) => {
            console.log(`proxy-stderr: ${data}`);
        });

        this.process.on('close', (code: number) => {
            console.log(`child process exited with code ${code}`);
        });
    }

    async getExecutablePath() {
        if (this.executablePath) return this.executablePath;

        const npmBin = await promisify(exec)("npm bin");
        if (npmBin.stderr || !npmBin.stdout) {
            throw Error("Unknown grooveproxy executable location")
        }
        const binDirectory = npmBin.stdout.trim();
        this.executablePath = join(binDirectory, "grooveproxy");

        // Determine if the path exists, will raise an error if not
        await stat(this.executablePath)

        return this.executablePath
    }
}

module.exports = Groove
