/**
 * LocalMind Core Binary Downloader
 *
 * Downloads the correct pre-built core engine binary from GitHub Releases
 * on first launch, caching it in the extension's global storage directory.
 * Subsequent launches use the cached binary directly.
 */

import * as https from 'https';
import * as fs from 'fs';
import * as path from 'path';
import * as vscode from 'vscode';

// =============================================================================
// Constants
// =============================================================================

const GITHUB_REPO = 'localmind/localmind';

// Maps Node.js platform/arch to the binary asset names on GitHub Releases.
const BINARY_NAMES: Record<string, string> = {
    'linux-x64': 'localmind-core-linux-amd64',
    'darwin-x64': 'localmind-core-darwin-amd64',
    'darwin-arm64': 'localmind-core-darwin-arm64',
    'win32-x64': 'localmind-core-windows-amd64.exe',
};

// =============================================================================
// Public API
// =============================================================================

/**
 * Ensures the core binary exists and is executable, downloading it if needed.
 * Returns the absolute path to the binary.
 *
 * In Development mode (F5 / Extension Development Host), skips download and
 * looks for a locally compiled binary instead.
 */
export async function ensureCoreBinary(
    context: vscode.ExtensionContext,
    version: string,
    onProgress?: (message: string) => void
): Promise<string> {
    // ── Dev mode: use locally compiled binary ────────────────────────────────
    if (context.extensionMode === vscode.ExtensionMode.Development) {
        const localBinary = findLocalDevBinary(context);
        if (localBinary) {
            onProgress?.(`[Dev] Using local binary: ${localBinary}`);
            return localBinary;
        }
        throw new Error(
            'LocalMind [Dev]: No compiled binary found. ' +
            'Run `go build -o localmind-core ./cmd/localmind` in packages/core first.'
        );
    }

    // ── Production: cached download from GitHub Releases ────────────────────
    const binaryDir = path.join(context.globalStorageUri.fsPath, 'bin', version);
    const binaryName = getPlatformBinaryName();
    const binaryPath = path.join(binaryDir, binaryName);

    // Already cached for this version
    if (fs.existsSync(binaryPath)) {
        onProgress?.('Core engine found in cache.');
        return binaryPath;
    }

    // Try to download from GitHub Releases
    try {
        const downloadUrl = await getDownloadUrl(version, binaryName);
        onProgress?.(`Downloading LocalMind core ${version} for ${process.platform}/${process.arch}...`);

        await fs.promises.mkdir(binaryDir, { recursive: true });
        await downloadFile(downloadUrl, binaryPath, onProgress);

        // Make executable on Unix
        if (process.platform !== 'win32') {
            await fs.promises.chmod(binaryPath, 0o755);
        }

        onProgress?.('Core engine ready.');
        return binaryPath;

    } catch (downloadErr: any) {
        // ── Fallback: look for a locally compiled binary ─────────────────────
        // This handles pre-release testing before a GitHub Release is tagged.
        if (downloadErr.message?.includes('404') || downloadErr.message?.includes('Not Found')) {
            onProgress?.('No GitHub release found — checking for local binary...');
            const localBinary = findLocalDevBinary(context);
            if (localBinary) {
                onProgress?.(`Using local binary: ${localBinary}`);
                return localBinary;
            }
            throw new Error(
                `LocalMind: No release found for version "${version}" on GitHub, ` +
                `and no local binary found at packages/core/localmind-core.\n\n` +
                `To run without a release:\n` +
                `  cd packages/core && go build -o localmind-core.exe ./cmd/localmind`
            );
        }
        throw downloadErr;
    }
}

/**
 * Returns the binary name for the current platform/architecture.
 * Throws an informative error if the platform is unsupported.
 */
export function getPlatformBinaryName(): string {
    const key = `${process.platform}-${process.arch}`;
    const name = BINARY_NAMES[key];
    if (!name) {
        throw new Error(
            `LocalMind: Unsupported platform: ${process.platform}-${process.arch}. ` +
            `Supported platforms: ${Object.keys(BINARY_NAMES).join(', ')}`
        );
    }
    return name;
}

// =============================================================================
// Private Helpers
// =============================================================================

/**
 * Searches for a locally compiled binary.
 * Checks paths relative to the extension root (dev mode) AND
 * relative to any open workspace folders (installed VSIX + monorepo open).
 */
function findLocalDevBinary(context: vscode.ExtensionContext): string | null {
    const ext = process.platform === 'win32' ? '.exe' : '';
    const binaryName = `localmind-core${ext}`;
    const extRoot = context.extensionUri.fsPath;

    // Paths relative to the extension install location
    const candidates: string[] = [
        path.join(extRoot, '..', '..', 'core', binaryName),   // monorepo: packages/extension → packages/core
        path.join(extRoot, '..', 'core', binaryName),          // flat layout
        path.join(extRoot, binaryName),                        // extension root itself
    ];

    // Also search inside any open workspace folders (handles installed VSIX case)
    const workspaceFolders = vscode.workspace.workspaceFolders ?? [];
    for (const folder of workspaceFolders) {
        const wsRoot = folder.uri.fsPath;
        candidates.push(
            path.join(wsRoot, 'packages', 'core', binaryName),  // monorepo root open
            path.join(wsRoot, 'core', binaryName),               // core subfolder open
            path.join(wsRoot, binaryName),                       // binary at workspace root
        );
    }

    for (const candidate of candidates) {
        if (fs.existsSync(candidate)) {
            return candidate;
        }
    }
    return null;
}

/**
 * Fetches the download URL for the binary from the GitHub Releases API.
 * Supports both exact version tags (v0.1.0) and "latest".
 */
async function getDownloadUrl(version: string, binaryName: string): Promise<string> {
    const releaseEndpoint = version === 'latest'
        ? `https://api.github.com/repos/${GITHUB_REPO}/releases/latest`
        : `https://api.github.com/repos/${GITHUB_REPO}/releases/tags/${version}`;

    const release = await fetchJson<GitHubRelease>(releaseEndpoint);
    const asset = release.assets.find(a => a.name === binaryName);

    if (!asset) {
        throw new Error(
            `LocalMind: Binary '${binaryName}' not found in GitHub Release '${version}'. ` +
            `Available assets: ${release.assets.map(a => a.name).join(', ')}`
        );
    }

    return asset.browser_download_url;
}

/**
 * Downloads a file from a URL to a local path, following redirects.
 */
function downloadFile(
    url: string,
    destPath: string,
    onProgress?: (msg: string) => void
): Promise<void> {
    return new Promise((resolve, reject) => {
        const doRequest = (requestUrl: string, redirectCount = 0) => {
            if (redirectCount > 5) {
                reject(new Error('Too many redirects while downloading binary'));
                return;
            }

            https.get(requestUrl, { headers: { 'User-Agent': 'LocalMind-Updater' } }, res => {
                // Follow redirects (GitHub assets redirect to S3)
                if (res.statusCode === 301 || res.statusCode === 302 || res.statusCode === 307) {
                    if (res.headers.location) {
                        doRequest(res.headers.location, redirectCount + 1);
                        return;
                    }
                }

                if (res.statusCode !== 200) {
                    reject(new Error(`Download failed with status ${res.statusCode}: ${requestUrl}`));
                    return;
                }

                const totalBytes = parseInt(res.headers['content-length'] || '0', 10);
                let downloadedBytes = 0;

                const file = fs.createWriteStream(destPath);
                res.on('data', chunk => {
                    downloadedBytes += chunk.length;
                    if (totalBytes > 0) {
                        const pct = Math.round((downloadedBytes / totalBytes) * 100);
                        onProgress?.(`Downloading... ${pct}%`);
                    }
                });
                res.pipe(file);

                file.on('finish', () => {
                    file.close();
                    resolve();
                });
                file.on('error', err => {
                    fs.unlink(destPath, () => { }); // Cleanup partial download
                    reject(err);
                });
            }).on('error', reject);
        };

        doRequest(url);
    });
}

/**
 * Makes an HTTPS GET request and parses the JSON response.
 */
function fetchJson<T>(url: string): Promise<T> {
    return new Promise((resolve, reject) => {
        https.get(url, { headers: { 'User-Agent': 'LocalMind-Updater', 'Accept': 'application/vnd.github.v3+json' } }, res => {
            let data = '';
            res.on('data', chunk => (data += chunk));
            res.on('end', () => {
                if (res.statusCode !== 200) {
                    reject(new Error(`GitHub API error ${res.statusCode}: ${data}`));
                    return;
                }
                try {
                    resolve(JSON.parse(data) as T);
                } catch (e) {
                    reject(new Error(`Failed to parse GitHub API response: ${e}`));
                }
            });
            res.on('error', reject);
        }).on('error', reject);
    });
}

// =============================================================================
// Types
// =============================================================================

interface GitHubRelease {
    tag_name: string;
    assets: GitHubAsset[];
}

interface GitHubAsset {
    name: string;
    browser_download_url: string;
}
