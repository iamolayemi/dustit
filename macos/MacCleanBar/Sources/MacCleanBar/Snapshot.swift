import AppKit
import SwiftUI

@MainActor
enum Snapshot {
    static func render(to dir: String) {
        for scheme in [ColorScheme.light, .dark] {
            let view = PanelView()
                .environmentObject(Model(sample: true))
                .background(Color(nsColor: .windowBackgroundColor))
                .environment(\.colorScheme, scheme)
            let renderer = ImageRenderer(content: view)
            renderer.scale = 2
            guard let image = renderer.nsImage, let tiff = image.tiffRepresentation,
                  let png = NSBitmapImageRep(data: tiff)?.representation(using: .png, properties: [:]) else { continue }
            let path = "\(dir)/panel-\(scheme == .dark ? "dark" : "light").png"
            try? png.write(to: URL(fileURLWithPath: path))
            print(path)
        }
    }
}
