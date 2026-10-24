/** Whether the browser runs on an iPhone or iPad, where installing goes through Safari's Share menu. */
export function isIos(userAgent: string, maxTouchPoints = 0): boolean {
  // iPads report themselves as Macs, but Macs have no touch screen
  return (
    /iphone|ipad|ipod/i.test(userAgent) || (/macintosh/i.test(userAgent) && maxTouchPoints > 1)
  );
}
