const { default: Groove, CacheModeEnum } = require("../index");
const HttpsProxyAgent = require('https-proxy-agent');
const { request } = require("https");

const httpsFetchWithCertificate = async (url: string, configuration: any) : Promise<string> => {
    return new Promise((resolve, reject) => {
        request(url, configuration, (response: any) => {
            let data = "";

            response.on("data", (chunk: any) => {
                data = data + chunk.toString();
            });
          
            response.on("end", () => {
                resolve(data);
            });

            response.on("error", (error: Error) => {
                reject(error);
            });
        }).end();
    });
}

describe('testing proxy client', () => {
    let proxy : typeof Groove | null = null;

    beforeAll(async () => {
        proxy = new Groove({});
        await proxy.launch();
    });
    
    afterAll(() => {
        proxy.stop();
    });

    test('tape should record and edit', async () => {
        await proxy.setCacheMode(CacheModeEnum.OFF);
        await proxy.tapeStart();

        var agent = new HttpsProxyAgent(proxy.baseUrlProxy);

        const contents = await httpsFetchWithCertificate(
            "https://freeman.vc", {
                agent,
                ca: proxy.certificate,
            }
        );
        expect(contents.length).toBeGreaterThanOrEqual(100);
        await proxy.tapeStop();

        const tape = await proxy.tapeGet();

        tape.records[0].response.body = Buffer.from("Hello world");
        await proxy.tapeLoad(tape);

        const contentsUpdated = await httpsFetchWithCertificate(
            "https://freeman.vc", {
                agent,
                ca: proxy.certificate,
            }
        )
        expect(contentsUpdated).toBe("Hello world");
    });
});
