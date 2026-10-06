# FitLog on iPhone and Mac

`/dashboard/summary` is a compact, numeric-only page. The web app manifest opens
it when FitLog is launched from an installed icon. The normal dashboard remains
available through the Overview link, including a separate exercise-progress
chart on the Training page.
For an actual numeric Home Screen widget via Scriptable, see
[the installation instructions](scriptable.md).

## Add an icon

Use the deployed HTTPS address of `/dashboard/summary`:

- **iPhone:** open in Safari, choose Share → Add to Home Screen. Enable Open as
  Web App if Safari shows that option.
- **Mac (macOS Sonoma or newer):** open in Safari, choose File → Add to Dock.

This uses Safari's web app support and requires no Xcode, signing, App Group or
Apple Developer membership. It opens a window with the summary; it is not a
system widget displaying live numbers on the desktop. The first launch may
require signing in with the existing dashboard token.

## Data and refresh

The page uses the existing HTTP-only dashboard session and the protected
`GET /api/v1/dashboard/summary` endpoint. No new token or browser token storage
is introduced for the web app. Scriptable has its own optional, read-only key.
The API returns `Cache-Control: no-store`; no service worker or
offline health-data cache is installed.

While the page is visible, it checks for updates every minute, refreshes on
focus, and has a manual refresh button. The displayed timestamp is the snapshot
time, not a claim that the external providers have synced at that moment.
Recovery, sleep and nutrition each show their own latest observed day from the
past 30 days. Body data uses the latest measurement even when it is older.
Training totals cover today and the previous six local days, using finished
sessions. Missing values appear as a dash; an actual zero remains zero.

An update failure keeps the last in-memory result with an explicit message;
an expired session hides it and offers sign-in. Internet access is required.
Installing this code locally does not update the deployed server.

Safari instructions: [iPhone](https://support.apple.com/guide/iphone/iphea86e5236/ios),
[Mac](https://support.apple.com/104996).
