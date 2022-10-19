import { gunzipSync, gzipSync } from "zlib";


interface TapeRequest {
    url: string
    method: string
    headers: Record<string, string[]>
    body: Buffer
}

interface TapeResponse {
    status: number
    headers: Record<string, string[]>
    body: Buffer
}

interface TapeRecord {
    request: TapeRequest
    response: TapeResponse
}

export class TapeSession {
    records: TapeRecord[];

    constructor() {
        this.records = [];
    }

    async readFromServer(contents: Buffer) {
        // un-gzip and un-json the blob
        const uncompressed = gunzipSync(contents);
        const jsonPayload = JSON.parse(uncompressed.toString());

        // Un-base64 the bodies
        this.records = jsonPayload.map((record: any) => {
            return {
                ...record,
                request: {
                    ...record.request,
                    body: Buffer.from(record.request.body, 'base64'),
                },
                response: {
                    ...record.response,
                    body: Buffer.from(record.response.body, 'base64'),
                }
            }
        });
    }

    toServer() : Buffer {
        // Base64 the bodies
        const jsonPayload = this.records.map((record) => {
            return {
                ...record,
                request: {
                    ...record.request,
                    body: record.request.body.toString('base64'),
                },
                response: {
                    ...record.response,
                    body: record.response.body.toString('base64'),
                }
            }
        });

        // Json and gzip the blob
        const compressed = gzipSync(JSON.stringify(jsonPayload));
        return compressed;
    }
}
