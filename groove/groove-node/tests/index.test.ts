const { Groove, CacheModeEnum } = require("../index");
const { fetchWithProxy } = require("../utilities");

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

        const contents = await fetchWithProxy(
            "https://freeman.vc", proxy
        );
        expect(contents.length).toBeGreaterThanOrEqual(100);
        await proxy.tapeStop();

        const tape = await proxy.tapeGet();

        tape.records[0].response.body = Buffer.from("Hello world");
        await proxy.tapeLoad(tape);

        const contentsUpdated = await fetchWithProxy(
            "https://freeman.vc", proxy
        )
        expect(contentsUpdated).toBe("Hello world");
    });
});
