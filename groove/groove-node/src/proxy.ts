import { promisify } from 'util';
import { exec, spawn } from 'child_process';
import { stat, realpath } from 'fs/promises';
import { readFileSync } from 'fs';
import { join } from 'path';
import { fetchWithTimeout, sleep, streamToBuffer } from './utilities';
import { TapeSession } from './tape';
import { homedir } from 'os';
import FormData from 'form-data';
import { DialerDefinition } from './dialer';

export const CacheModeEnum = {
	OFF: 0,
    STANDARD: 1,
    AGGRESSIVE_GET: 2,
    AGGRESSIVE: 3
}

export interface GrooveConfiguration {
    commandTimeout?: number;
    port?: number;
    controlPort?: number;
    authUsername?: string;
    authPassword?: string;
}

export interface EndProxyOptions {
    server: string;
    username?: string;
    password?: string;
}

const checkStatus = async (response: any, echoError: string) => {
    if (response.status > 300 || response.status < 200) {
        console.log(`Error: ${response.status}: ${await response.text()}`)
        throw Error(echoError)
    }

    const contents = await response.json() as any;
    if (contents["success"] != true) {
        throw Error(echoError)
    }
}

export const getExecutable = async () => {
    const npmBin = await promisify(exec)("npm bin");
    if (npmBin.stderr || !npmBin.stdout) {
        throw Error("Unknown grooveproxy executable location")
    }
    const binDirectory = npmBin.stdout.trim();
    let executablePath = join(binDirectory, "grooveproxy");

    // Resolve symbolic links
    executablePath = await realpath(executablePath)

    // Determine if the path exists, will raise an error if not
    await stat(executablePath)

    return executablePath
}

export class Groove {
    process: any
    executablePath: string | null

    commandTimeout: number
    port: number
    controlPort: number
    authUsername: string | null
    authPassword: string | null

    baseUrlProxy: string;
    baseUrlControl: string;

    certificate: Buffer;

    // If true, will not pass through logs. Default to false.
    silenceLogs: boolean;

    // If true, we have already launched the proxy server.
    launched: boolean;

    constructor(config: GrooveConfiguration) {
        this.process = null;
        this.executablePath = null;

        this.commandTimeout = config.commandTimeout || 5000;
        this.port = config.port || 6010;
        this.controlPort = config.controlPort || 6011;
        this.authUsername = config.authUsername || null;
        this.authPassword = config.authPassword || null;

        this.baseUrlProxy = `http://localhost:${this.port}`
        this.baseUrlControl = `http://localhost:${this.controlPort}`

        this.certificate = readFileSync(join(homedir(), '.grooveproxy/ca.crt'));

        this.silenceLogs = false;
        this.launched = false;
    }

    async launch() {
        if (this.launched) {
            throw Error("Already spawned proxy.")
        }

        const parameters = {
            "--port": this.port ? this.port.toString() : null,
            "--control-port": this.controlPort ? this.controlPort.toString() : null,
            "--auth-username": this.authUsername,
            "--auth-password": this.authPassword,
        }

        const exc = await this.getExecutablePath()
        console.log(`Will launch groove executable: ${exc}`)
        this.process = spawn(
            exc,
            Object.entries(parameters).reduce((previous: string[], [key, value] : [string, string | null]) => {    
                if (value != null) {
                    previous = [...previous, key, value]
                }
                return previous;
            }, [])
        );
        this.launched = true;

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
        this.launched = false;
        this.process = null;
    }

    async tapeStart() {
        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/tape/record`,
            {
                method: "POST",
                timeout: this.commandTimeout,
            }
        )
        await checkStatus(response, "Failed to start recording.");
    }

    async tapeGet(tapeID?: string) : Promise<TapeSession> {
        tapeID = tapeID || "";

        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/tape/retrieve`,
            {
                method: "POST",
                timeout: this.commandTimeout,
                body: JSON.stringify({
                    tapeID,
                }),
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
        await checkStatus(response, "Failed to load tape.");
    }

    async tapeStop() {
        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/tape/stop`,
            {
                method: "POST",
                timeout: this.commandTimeout,
            }
        )
        await checkStatus(response, "Failed to stop recording.");
    }

    async tapeClear(tapeID?: string) {
        tapeID = tapeID || "";
        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/tape/clear`,
            {
                method: "POST",
                timeout: this.commandTimeout,
                body: JSON.stringify({
                    tapeID,
                }),
            }
        )
        await checkStatus(response, "Failed to clear tape.");
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
        await checkStatus(response, "Failed to set cache mode.");
    }

    async cacheClear() {
        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/cache/clear`,
            {
                method: "POST",
                timeout: this.commandTimeout,
            }
        )
        await checkStatus(response, "Failed to clear cache.");
    }

    async dialerLoad(dialers: DialerDefinition[]) {
        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/dialer/load`,
            {
                method: "POST",
                timeout: this.commandTimeout,
                body: JSON.stringify(
                    {
                        definitions: dialers.map(
                            (dialer) => ({
                                priority: dialer.priority,
                                proxyServer: dialer.proxy ? dialer.proxy.url : null,
                                proxyUsername: dialer.proxy ? dialer.proxy.username : null,
                                proxyPassword: dialer.proxy ? dialer.proxy.password : null,
                                requiresUrlRegex: dialer.requestRequires ? dialer.requestRequires.urlRegex : null,
                                requiresResourceTypes: dialer.requestRequires ? dialer.requestRequires.resourceTypes : null,
                            })
                        )
                    }
                ),
            }
        )

        await checkStatus(response, "Failed to start end-proxy.")
    }

    async endProxyStop() {
        const response = await fetchWithTimeout(
            `${this.baseUrlControl}/api/proxy/stop`,
            {
                method: "POST",
                timeout: this.commandTimeout,
            }
        )
        await checkStatus(response, "Failed to stop end-proxy.")
    }

    async getExecutablePath() {
        if (this.executablePath) return this.executablePath;

       this.executablePath = await getExecutable();
       return this.executablePath;
    }
}
