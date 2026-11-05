/** Hands a downloaded file to the browser. */
export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  // Not at once: the browser reads the URL after click() returns, and some
  // revoke it from under the download.
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
