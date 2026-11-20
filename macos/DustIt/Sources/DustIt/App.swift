import AppKit
import SwiftUI

@main
struct DustItApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @StateObject private var model = Model.shared

    init() {
        // `DustIt --snapshot <dir>` renders the panel with sample data (light + dark) and exits.
        let args = CommandLine.arguments
        if let i = args.firstIndex(of: "--login-item"), i + 1 < args.count {
            // `DustIt --login-item on|off|status` manages Open at Login from the command line.
            if args[i + 1] != "status", let err = LoginItem.set(args[i + 1] == "on") {
                print("error: \(err)")
            }
            print("Open at Login: \(LoginItem.statusText)")
            exit(0)
        }
        if let i = args.firstIndex(of: "--iconset"), i + 1 < args.count {
            // `DustIt --iconset Foo.iconset` writes every size iconutil needs for the .icns.
            try? FileManager.default.createDirectory(atPath: args[i + 1], withIntermediateDirectories: true)
            for (pt, scale) in [(16, 1), (16, 2), (32, 1), (32, 2), (128, 1), (128, 2), (256, 1), (256, 2), (512, 1), (512, 2)] {
                let name = scale == 1 ? "icon_\(pt)x\(pt).png" : "icon_\(pt)x\(pt)@2x.png"
                Snapshot.renderAppIcon(to: "\(args[i + 1])/\(name)", size: CGFloat(pt * scale))
            }
            exit(0)
        }
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
