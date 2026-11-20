# Use NSGo's signed native updates

jisu uses NSGo's direct update channel: Sparkle on macOS and WinSparkle on Windows. They verify EdDSA-signed full packages, install and restart using their own dialogs. Settings exposes the current version and a manual check; native daily checks need no custom scheduler or installer code.

The public key lives in nsgo.yaml; the matching secret stays in the release environment as NSGO_UPDATES_KEY. CI publishes all three target appcasts with each release and refuses a tagged release missing one, because the default feed follows GitHub's latest release. Checksums remain available for manual downloads. v0.2.0 users must install the first updater-enabled release manually.

Ad hoc macOS signing remains consistent with ADR-0007. NSGo documents a successful ad hoc Sparkle update; Developer ID and notarization are not required for this channel. Native end-to-end installation still needs verification on each supported OS.
