export async function load() {

    const sv = `${process.env.SVEBAPP_SERVER_HOST}:${process.env.SVEBAPP_SERVER_PORT}`

    try {
        const data = await fetch(`http://${sv}/items`);
        return data.json();
    } catch (err) {
        console.log(err);
    }

    return {};
}
