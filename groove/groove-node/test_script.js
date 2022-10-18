const Groove = require("./dist/index.js");

const main = async () => {
    const proxy = new Groove({});
    await proxy.launch();

    const delay = ms => new Promise(resolve => setTimeout(resolve, ms))
    await delay(10000)

    console.log("Will close")
}

main()
