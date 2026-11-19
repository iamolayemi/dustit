import AppKit
import SwiftUI

@main
struct MacCleanBarApp: App {
    @StateObject private var model = Model()

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
