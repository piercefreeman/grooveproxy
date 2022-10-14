const { chromium } = require('playwright');

const main = async () => {
    const browser = await chromium.launch({
        headless: false,
    });

    const context = await browser.newContext({
    });

    // Apply to everything in the context - popups, opened links, etc.
    // TODO: Apply to webworkers and other outgoing data requests
    await context.route('**/api/fetch_data', route => {
        route.fulfill({
            status: 200,
            body: "something",
        });
    });

    const page = await context.newPage();

    //await page.goto("https://freeman.vc")
    await page.goto("https://freeman.vc/api/fetch_data")
}

main();
