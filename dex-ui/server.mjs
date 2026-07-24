// Server statis zero-dependency untuk Orderbox Salvage Console.
// Jalankan: node server.mjs   →   http://localhost:8080
import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { extname, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('.', import.meta.url));
const types = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.mjs': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json',
  '.md': 'text/markdown; charset=utf-8',
  '.png': 'image/png',
  '.svg': 'image/svg+xml',
};
const port = Number(process.env.PORT || 8080);

createServer(async (req, res) => {
  try {
    let path = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);
    if (path === '/') path = '/index.html';
    const file = normalize(join(root, path));
    if (!file.startsWith(root)) throw new Error('di luar root');
    const data = await readFile(file);
    res.writeHead(200, { 'content-type': types[extname(file).toLowerCase()] || 'application/octet-stream' });
    res.end(data);
  } catch {
    res.writeHead(404, { 'content-type': 'text/plain; charset=utf-8' });
    res.end('404 — berkas tidak ditemukan');
  }
}).listen(port, () => {
  console.log(`Orderbox Salvage Console → http://localhost:${port}`);
});
