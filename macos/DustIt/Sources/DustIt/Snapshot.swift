import AppKit
import SwiftUI

@MainActor
enum Snapshot {
    /// Renders the menu bar icon large (and at real size) on light and dark menu bars.
    static func renderIcon(to dir: String) {
        for (name, bg, fg) in [("light", NSColor(white: 0.93, alpha: 1), NSColor.black), ("dark", NSColor(white: 0.16, alpha: 1), NSColor.white)] {
            let scale: CGFloat = 16
            let w = Icon.size.width * scale * 2 + 80, h = Icon.size.height * scale + 40
            let img = NSImage(size: NSSize(width: w, height: h), flipped: false) { rect in
                bg.setFill(); rect.fill()
                for (i, low) in [false, true].enumerated() {
                    let tile = NSImage(size: Icon.size, flipped: true) { _ in Icon.draw(low: low); return true }
                    let tinted = NSImage(size: Icon.size, flipped: false) { r in
                        tile.draw(in: r)
                        fg.set()
                        r.fill(using: .sourceAtop)
                        return true
                    }
                    tinted.draw(in: NSRect(x: 20 + CGFloat(i) * (Icon.size.width * scale + 40), y: 20,
                                           width: Icon.size.width * scale, height: Icon.size.height * scale))
                    tinted.draw(in: NSRect(x: w - 60 + CGFloat(i) * 24, y: h - 30, width: 18, height: 18)) // real size
                }
                return true
            }
            if let tiff = img.tiffRepresentation, let png = NSBitmapImageRep(data: tiff)?.representation(using: .png, properties: [:]) {
                let path = "\(dir)/icon-\(name).png"
                try? png.write(to: URL(fileURLWithPath: path))
                print(path)
            }
        }
    }

    /// The app icon: the broom glyph in white on a deep teal rounded square.
    static func renderAppIcon(to path: String, size: CGFloat) {
        let img = NSImage(size: NSSize(width: size, height: size), flipped: false) { rect in
            let inset = rect.insetBy(dx: size * 0.06, dy: size * 0.06)
            let bg = NSBezierPath(roundedRect: inset, xRadius: size * 0.2, yRadius: size * 0.2)
            NSGradient(starting: NSColor(red: 0.16, green: 0.62, blue: 0.56, alpha: 1),
                       ending: NSColor(red: 0.07, green: 0.36, blue: 0.40, alpha: 1))?.draw(in: bg, angle: -90)
            let glyph = NSImage(size: Icon.size, flipped: true) { _ in Icon.draw(low: false); return true }
            let white = NSImage(size: Icon.size, flipped: false) { r in
                glyph.draw(in: r)
                NSColor.white.set()
                r.fill(using: .sourceAtop)
                return true
            }
            let g = size * 0.56
            white.draw(in: NSRect(x: (size - g) / 2, y: (size - g) / 2, width: g, height: g))
            return true
        }
        if let tiff = img.tiffRepresentation, let png = NSBitmapImageRep(data: tiff)?.representation(using: .png, properties: [:]) {
            try? png.write(to: URL(fileURLWithPath: path))
            print(path)
        }
    }

    static func render(to dir: String) {
        renderIcon(to: dir)
        renderAppIcon(to: "\(dir)/app-icon.png", size: 512)
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
