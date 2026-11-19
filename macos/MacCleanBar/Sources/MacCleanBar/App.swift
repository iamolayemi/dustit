import AppKit
import SwiftUI

@main
struct MacCleanBarApp: App {
    @StateObject private var model = Model()

    init() {
        // `MacCleanBar --snapshot <dir>` renders the panel with sample data (light + dark) and exits.
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
            Image(systemName: model.isLow ? "externaldrive.badge.exclamationmark" : "internaldrive")
            if model.showSizeInMenuBar || model.isLow {
                Text(shortSize(model.free))
            }
        }
        .menuBarExtraStyle(.window)
    }
}
