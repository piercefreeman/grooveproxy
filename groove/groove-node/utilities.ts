import fetch, { RequestInit } from 'node-fetch';

interface FetchTimeoutConfiguration extends RequestInit {
    timeout?: number;
}

export const fetchWithTimeout = async (url: string, options: FetchTimeoutConfiguration) => {
    const { timeout } = options;

    const controller = new AbortController();
    const id = setTimeout(() => controller.abort(), timeout);

    const response = await fetch(url, {
        ...options,
        //signal: controller.signal,
        // Non-standard timeout parameter specified in node-fetch
        // We use this instead of the signal because the 2.x.x signal type definitions
        // are incompatible. They were fixed in 3.x but 3.x also brought along ESM only requirements
        // that we don't currently use.
        timeout,
    });

    clearTimeout(id);
    return response;
}

export const sleep = (ms: number) => {
    return new Promise(resolve => setTimeout(resolve, ms))
}

export const streamToBuffer = (stream: any) : Promise<Buffer> => {
    // Right now we can't typehint the stream because node's built-in ReadableStream
    // typehint is missing `.on` properties
    return new Promise((resolve, reject) => {
        
        const _buf: Uint8Array[] = [];

        stream.on("data", (chunk: Uint8Array) => _buf.push(chunk));
        stream.on("end", () => resolve(Buffer.concat(_buf)));
        stream.on("error", (err: Error) => reject(err));
    });
} 
