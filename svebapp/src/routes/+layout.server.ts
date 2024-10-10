export async function load() {

    try {
        const data = await fetch('http://localhost:3000/items');
        return data.json();
    } catch (err) {
        console.log(err);
    }

    return {};
}
