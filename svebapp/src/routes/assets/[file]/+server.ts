import fs from 'fs';
import path from 'path';

export async function GET({ params }) {
    console.log("Loading file from +server.ts")
    const file = params.file;
    const filePath = path.join('', file);

    try {
        const image = await fs.promises.readFile(filePath.replace('%20', ' '));
        return new Response(image/*, {
            headers: { 'Content-Type': 'image/jpeg' }
        }*/);
    } catch (error) {
        return new Response('File not found', { status: 404 });
    }
}
