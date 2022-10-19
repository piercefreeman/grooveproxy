import { promisify } from 'util';
import { exec, spawn } from 'child_process';
import { stat } from 'fs/promises';
import { readFileSync } from 'fs';
import { join } from 'path';
import { fetchWithTimeout, sleep, streamToBuffer } from './utilities';
import { TapeSession } from './tape';
import { homedir } from 'os';
import FormData from 'form-data';

export const CacheModeEnum = {
	OFF: 0,
    STANDARD: 1,
    AGGRESSIVE: 2
}

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

    baseUrlProxy: string;
    baseUrlControl: string;

    certificate: Buffer;

    // If true, will not pass through logs. Default to false.
    silenceLogs: boolean;

    constructor(config: GrooveConfiguration) {
        this.process = null;
        this.executablePath = null;

        this.commandTimeout = config.commandTimeout || 5000;
        this.port = config.port || 6010;
        this.controlPort = config.controlPort || 6011;
        this.proxyServer = config.proxyServer || null;
        this.proxyUsername = config.proxyUsername || null;
        this.proxyPassword = config.proxyPassword || null;
        this.authUsername = config.authUsername || null;
        this.authPassword = config.authPassword || null;
        this.silenceLogs = false;

        this.baseUrlProxy = `http://localhost:${this.port}`
        this.baseUrlControl = `http://localhost:${this.controlPort}`

        this.certificate = readFileSync(join(homedir(), '.grooveproxy/ca.crt'));
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
            if (this.silenceLogs) return;
            console.log(`proxy-stdout: ${data}`);
        });

        this.process.stderr.setEncoding('utf8');
        this.process.stderr.on('data', (data: string) => {
            if (this.silenceLogs) return;
            console.log(`proxy-stderr: ${data}`);
        });

        this.process.on('close', (code: number) => {
            if (this.silenceLogs) return;
            console.log(`child process exited with code ${code}`);
        });

        // Ensure we cleanup after ourselves
        process.on('exit', () => {
            console.log("Will quit")
            this.stop();
        });

        await sleep(1000);
    }

    async stop() {
        this.silenceLogs = true;
        if (this.process) this.process.kill();
    }

    async tapeStart() {
        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/tape/record`,
            {
                method: "POST",
                timeout: this.commandTimeout,
            }
        )
        const contents = await response.json() as any;
        if (contents["success"] != true) {
            throw Error("Failed to start recording.")
        }
    }

    async tapeGet() : Promise<TapeSession> {
        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/tape/retrieve`,
            {
                method: "POST",
                timeout: this.commandTimeout,
            }
        )
        const contents = response.body;
        const session = new TapeSession();
        session.readFromServer(await streamToBuffer(contents))
        return session;
    }

    async tapeLoad(session: TapeSession) {
        const formData = new FormData();
        // Filename is irrelevant but required to upload payload buffer as file
        formData.append("file", session.toServer(), { filename : 'tape.json.gzip' });

        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/tape/load`,
            {
                method: "POST",
                timeout: this.commandTimeout,
                body: formData,
            }
        )
        const contents = await response.json() as any;
        if (contents["success"] != true) {
            throw Error("Failed to load tape.")
        }
    }

    async tapeStop() {
        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/tape/stop`,
            {
                method: "POST",
                timeout: this.commandTimeout,
            }
        )
        const contents = await response.json() as any;
        if (contents["success"] != true) {
            throw Error("Failed to stop recording.")
        }
    }

    async setCacheMode(mode: number) {
        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/cache/mode`,
            {
                method: "POST",
                timeout: this.commandTimeout,
                body: JSON.stringify({ mode }),
            }
        )
        const contents = await response.json() as any;
        if (contents["success"] != true) {
            throw Error("Failed to set cache mode.")
        }
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

export default Groove;
