import AppKit
import SwiftUI

@main
struct DustItApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @StateObject private var model = Model.shared

    init() {
        // `DustIt --snapshot <dir>` renders the panel with sample data (light + dark) and exits.
        let args = CommandLine.arguments
        if let i = args.firstIndex(of: "--snapshot"), i + 1 < args.count {
            Snapshot.render(to: args[i + 1])
            exit(0)
        }
    }

    var body: some Scene {
        MenuBarExtra {
            PanelView()
                .environmentObject(model)
                .onAppear { model.panelOpened() }
        } label: {
            Image(nsImage: Icon.menuBar(low: model.isLow))
            if model.showSizeInMenuBar || model.isLow {
                Text(shortSize(model.free))
            }
        }
        .menuBarExtraStyle(.window)
    }
}

/// Handles dustit:// links (from Shortcuts' "Open URL", Raycast or a browser).
final class AppDelegate: NSObject, NSApplicationDelegate {
    func application(_ application: NSApplication, open urls: [URL]) {
        for url in urls {
            Task { @MainActor in Model.shared.handle(url) }
        }
    }
}
