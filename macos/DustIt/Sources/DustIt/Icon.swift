import AppKit

/// DustIt's menu bar icon: a broom sweeping up a sparkle of dust. It's drawn as vector code
/// and marked as a template image, so macOS tints it like its own menu bar icons (black in
/// light mode, white in dark mode and when highlighted).
enum Icon {
    /// 18×18 pt, the standard menu bar glyph size.
    static let size = NSSize(width: 18, height: 18)

    static func menuBar(low: Bool = false) -> NSImage {
        let image = NSImage(size: size, flipped: true) { _ in
            draw(low: low)
            return true
        }
        image.isTemplate = true
        image.accessibilityDescription = low ? "DustIt — disk almost full" : "DustIt"
        return image
    }

    /// Draws into the current context in an 18×18 coordinate space (y grows downwards).
    static func draw(low: Bool) {
        NSColor.black.setFill()
        NSColor.black.setStroke()

        // Handle: a straight stick from the top right down to the head.
        let handle = NSBezierPath()
        handle.move(to: NSPoint(x: 15.6, y: 1.6))
        handle.line(to: NSPoint(x: 10.1, y: 7.1))
        handle.lineWidth = 1.7
        handle.lineCapStyle = .round
        handle.stroke()

        // Ferrule: the band that holds the bristles.
        let ferrule = NSBezierPath()
        ferrule.move(to: NSPoint(x: 8.2, y: 6.0))
        ferrule.line(to: NSPoint(x: 11.4, y: 9.2))
        ferrule.lineWidth = 2.6
        ferrule.lineCapStyle = .round
        ferrule.stroke()

        // Bristles: a fan sweeping down to the left.
        let head = NSBezierPath()
        head.move(to: NSPoint(x: 7.4, y: 7.6))
        head.line(to: NSPoint(x: 10.0, y: 10.2))
        head.curve(to: NSPoint(x: 7.6, y: 15.8),
                   controlPoint1: NSPoint(x: 9.9, y: 12.4), controlPoint2: NSPoint(x: 9.0, y: 14.4))
        head.line(to: NSPoint(x: 2.0, y: 10.4))
        head.curve(to: NSPoint(x: 7.4, y: 7.6),
                   controlPoint1: NSPoint(x: 3.4, y: 8.9), controlPoint2: NSPoint(x: 5.4, y: 7.9))
        head.close()
        head.fill()

        // Bristle grooves, cut out of the fan.
        if let ctx = NSGraphicsContext.current {
            ctx.saveGraphicsState()
            ctx.compositingOperation = .destinationOut
            for (from, to) in [(NSPoint(x: 6.6, y: 10.0), NSPoint(x: 4.4, y: 12.4)),
                               (NSPoint(x: 8.0, y: 11.4), NSPoint(x: 5.9, y: 13.9))] {
                let groove = NSBezierPath()
                groove.move(to: from)
                groove.line(to: to)
                groove.lineWidth = 0.9
                groove.lineCapStyle = .round
                groove.stroke()
            }
            ctx.restoreGraphicsState()
        }

        if low {
            // Disk almost full: a solid dot where the sparkle would be.
            NSBezierPath(ovalIn: NSRect(x: 12.6, y: 11.6, width: 4.6, height: 4.6)).fill()
        } else {
            // A four-point sparkle: the dust being swept away.
            sparkle(center: NSPoint(x: 14.6, y: 13.4), radius: 3.0)
            NSBezierPath(ovalIn: NSRect(x: 11.4, y: 15.6, width: 1.6, height: 1.6)).fill()
        }
    }

    private static func sparkle(center c: NSPoint, radius r: CGFloat) {
        let k = r * 0.28 // pinch of the curves
        let p = NSBezierPath()
        p.move(to: NSPoint(x: c.x, y: c.y - r))
        p.curve(to: NSPoint(x: c.x + r, y: c.y), controlPoint1: NSPoint(x: c.x + k, y: c.y - k), controlPoint2: NSPoint(x: c.x + k, y: c.y - k))
        p.curve(to: NSPoint(x: c.x, y: c.y + r), controlPoint1: NSPoint(x: c.x + k, y: c.y + k), controlPoint2: NSPoint(x: c.x + k, y: c.y + k))
        p.curve(to: NSPoint(x: c.x - r, y: c.y), controlPoint1: NSPoint(x: c.x - k, y: c.y + k), controlPoint2: NSPoint(x: c.x - k, y: c.y + k))
        p.curve(to: NSPoint(x: c.x, y: c.y - r), controlPoint1: NSPoint(x: c.x - k, y: c.y - k), controlPoint2: NSPoint(x: c.x - k, y: c.y - k))
        p.close()
        p.fill()
    }
}
