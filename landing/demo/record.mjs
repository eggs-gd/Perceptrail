// The perceptors' demo for the landing and the README: a gallery over the demo library
// (demo/README.md), recorded in a headless Chrome. A cursor and the perceptor's own
// hint are drawn over the page (a headless recording has neither): the cursor marks a
// photo, goes to a perceptor's button, its hint shows, a click — the sheet is ordered
// anew around the marked photo, which stays in view.
//
//   node demo/record.mjs <gallery url> <out dir>
// Writes <out>/raw.mp4 (the recording) and <out>/poster.png (the first view).
//
// Not obvious: the frames come from Chrome's own screencast (CDP), each with its time,
// and the system's ffmpeg makes them a video at a steady rate — Playwright's video needs
// its own ffmpeg build.
import {chromium} from 'playwright-core';
import {execFileSync} from 'node:child_process';
import {mkdirSync, writeFileSync} from 'node:fs';
import {join} from 'node:path';

const [gallery, out] = process.argv.slice(2);
if (!gallery || !out) throw new Error('usage: node demo/record.mjs <gallery url> <out dir>');
mkdirSync(out, {recursive: true});

// The story: where it starts, then the views clicked in turn
const start = 'size';
const steps = ['Date', 'Place'];
const size = {width: 1280, height: 800};

const browser = await chromium.launch({channel: 'chrome', headless: true});
const context = await browser.newContext({viewport: size});
const page = await context.newPage();
await page.goto(new URL(`/v/${start}`, gallery).href);
await page.waitForLoadState('networkidle');
await page.waitForTimeout(2500);
await page.screenshot({path: join(out, 'poster.png')});

// The camera: every frame Chrome paints, kept with its time
const frames = [];
const cdp = await context.newCDPSession(page);
cdp.on('Page.screencastFrame', async ({data, metadata, sessionId}) => {
    frames.push({data, time: metadata.timestamp});
    await cdp.send('Page.screencastFrameAck', {sessionId}).catch(() => {});
});
await cdp.send('Page.startScreencast', {format: 'jpeg', quality: 92, maxWidth: size.width, maxHeight: size.height});

// Into the sheet: at its very top a switch keeps the top, not a photo (svebapp's README)
await page.evaluate(() => window.scrollTo({top: innerHeight * 1.6, behavior: 'smooth'}));
await page.waitForTimeout(1800);


// The overlay: a cursor, a ring on the marked photo, a hint bubble
await page.evaluate(() => {
    const css = document.createElement('style');
    css.textContent = `
      .demo-cursor { position: fixed; z-index: 9999; width: 22px; height: 22px; pointer-events: none;
        transition: left .9s cubic-bezier(.45,.05,.25,1), top .9s cubic-bezier(.45,.05,.25,1); }
      .demo-cursor::before { content: ''; position: absolute; inset: 0; border-radius: 50%;
        background: rgba(255,179,71,.95); box-shadow: 0 0 0 4px rgba(255,179,71,.3), 0 2px 8px rgba(0,0,0,.5); }
      .demo-cursor.click::after { content: ''; position: absolute; inset: -14px; border-radius: 50%;
        border: 3px solid rgba(255,179,71,.9); animation: demo-ripple .5s ease-out forwards; }
      @keyframes demo-ripple { from { transform: scale(.4); opacity: 1 } to { transform: scale(1.4); opacity: 0 } }
      .demo-ring { position: fixed; z-index: 9998; pointer-events: none; border: 4px solid #ffb347; border-radius: 6px;
        box-shadow: 0 0 0 9999px rgba(0,0,0,0); transition: all .6s ease; opacity: 0; }
      .demo-hint { position: fixed; z-index: 9999; max-width: 300px; padding: 10px 14px; border-radius: 10px;
        background: #1b1f3b; color: #eceef7; font: 14px/1.4 system-ui, sans-serif; border: 1px solid rgba(255,255,255,.15);
        box-shadow: 0 8px 24px rgba(0,0,0,.45); opacity: 0; transform: translateY(-6px); transition: opacity .3s, transform .3s; }
      .demo-hint b { display: block; color: #ffb347; margin-bottom: 2px; }
      .demo-hint.on { opacity: 1; transform: none; }`;
    document.head.append(css);
    for (const c of ['demo-cursor', 'demo-ring', 'demo-hint']) {
        const el = document.createElement('div');
        el.className = c;
        document.body.append(el);
    }
    Object.assign(document.querySelector('.demo-cursor').style, {left: '640px', top: '760px'});
});

const pause = (ms) => page.waitForTimeout(ms);
const cursorTo = async (x, y) => {
    await page.evaluate(([x, y]) => Object.assign(document.querySelector('.demo-cursor').style, {left: `${x - 11}px`, top: `${y - 11}px`}), [x, y]);
    await pause(1000);
};
const click = async () => {
    await page.evaluate(() => {
        const c = document.querySelector('.demo-cursor');
        c.classList.remove('click');
        void c.offsetWidth;
        c.classList.add('click');
    });
};

// The marked photo: the one under the middle of the screen; the ring follows it by its id
const marked = await page.evaluate(() => {
    const img = document.elementFromPoint(innerWidth / 2, innerHeight / 2)?.closest('.asset, picture')?.querySelector('img');
    return img?.src.match(/assets\/([0-9a-f-]{36})/)?.[1];
});
const ringOn = async () => {
    // After a relayout the tile comes back before its image: tried for a while; of
    // two images of the photo (the old place leaving), the one on screen
    for (let tries = 0; tries < 20; tries++) {
        const at = await page.evaluate((guid) => {
            const shown = [...document.querySelectorAll('img')].filter((i) => i.src.includes(guid))
                .map((i) => i.getBoundingClientRect()).filter((r) => r.width > 0 && r.bottom > 0 && r.top < innerHeight);
            const r = shown[0];
            if (!r) return null;
            Object.assign(document.querySelector('.demo-ring').style, {left: `${r.left - 4}px`, top: `${r.top - 4}px`, width: `${r.width + 8}px`, height: `${r.height + 8}px`, opacity: '1'});
            return {x: r.left + r.width / 2, y: r.top + r.height / 2};
        }, marked);
        if (at) return at;
        await page.waitForTimeout(150);
    }
    return null;
};

const centre = await ringOn();
if (centre) {
    await cursorTo(centre.x, centre.y);
    await pause(900);
}

// Each view: to its button, its hint (the perceptor's own help), a click, the new order
const perceptors = await page.evaluate(async () => (await fetch(new URL('/api/perceptors', location.href)).then((r) => r.json())));
for (const title of steps) {
    const button = page.getByRole('button', {name: title});
    const box = await button.boundingBox();
    await cursorTo(box.x + box.width / 2, box.y + box.height / 2);
    const help = perceptors.find((p) => p.title === title)?.help ?? '';
    await page.evaluate(([title, help, box]) => {
        const hint = document.querySelector('.demo-hint');
        hint.innerHTML = '';
        const b = document.createElement('b');
        b.textContent = title;
        hint.append(b, help);
        Object.assign(hint.style, {top: `${box.y + box.height + 12}px`, left: `${Math.max(16, box.x + box.width - 300)}px`});
        hint.classList.add('on');
    }, [title, help, box]);
    await pause(1800);
    await click();
    await button.click();
    await pause(400);
    await page.evaluate(() => document.querySelector('.demo-hint').classList.remove('on'));
    await pause(3000); // the new order lays out in a wave from the marked photo
    const at = await ringOn();
    if (at) await cursorTo(at.x, at.y);
    await pause(2200);
}

await cdp.send('Page.stopScreencast');
await browser.close();

// The frames as a video: each shown until the next one came (ffconcat durations)
const dir = join(out, 'frames');
mkdirSync(dir, {recursive: true});
frames.sort((a, b) => a.time - b.time); // acknowledged out of order
let list = 'ffconcat version 1.0\n';
frames.forEach((f, i) => {
    const file = join(dir, `${String(i).padStart(5, '0')}.jpg`);
    writeFileSync(file, Buffer.from(f.data, 'base64'));
    const next = frames[i + 1]?.time ?? f.time + 2.5; // the last one held: frames come only on a change
    list += `file '${file}'\nduration ${Math.max(next - f.time, 0.001).toFixed(3)}\n`;
});
// concat takes the last entry's duration only if a file follows it: the last again
list += `file '${join(dir, `${String(frames.length - 1).padStart(5, '0')}.jpg`)}'\n`;
writeFileSync(join(out, 'frames.txt'), list);
execFileSync('ffmpeg', ['-loglevel', 'error', '-y', '-f', 'concat', '-safe', '0', '-i', join(out, 'frames.txt'),
    '-vf', 'fps=30,format=yuv420p', '-c:v', 'libx264', '-crf', '16', join(out, 'raw.mp4')]);
console.log('recorded', join(out, 'raw.mp4'), frames.length, 'frames');
