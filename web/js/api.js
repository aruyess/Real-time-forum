// Parse the fetch Response into a JSON body, throwing Error with .status
// on non-2xx. Shared between the JSON and multipart helpers below.
async function parseResponse(res) {
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
        const err = new Error(body.error || `HTTP ${res.status}`);
        err.status = res.status;
        throw err;
    }
    return body;
}

// Thin fetch wrapper. Always sends cookies (credentials: same-origin)
// and throws Error with .status on non-2xx.
async function request(method, url, data) {
    const opts = {
        method,
        credentials: "same-origin",
    };
    if (data !== undefined) {
        opts.headers = { "Content-Type": "application/json" };
        opts.body = JSON.stringify(data);
    }
    return parseResponse(await fetch(url, opts));
}

// Multipart POST (used for image uploads). Bypasses the JSON request helper
// because the body is a FormData and the Content-Type header is set by the
// browser to include the multipart boundary.
async function upload(url, formData) {
    const res = await fetch(url, {
        method: "POST",
        body: formData,
        credentials: "same-origin",
    });
    return parseResponse(res);
}

export const api = {
    get:    (url)       => request("GET",  url),
    post:   (url, data) => request("POST", url, data ?? {}),
    put:    (url, data) => request("PUT",  url, data ?? {}),
    del:    (url)       => request("DELETE", url),
    upload: (url, fd)   => upload(url, fd),
};
