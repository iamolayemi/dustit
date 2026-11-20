import SwiftUI

/// The panel follows macOS's own menu bar extras (Wi-Fi, Battery): system font, menu-item
/// rows with the standard highlight, small section headers, numbers aligned right, and
/// colour only where it carries meaning.
struct PanelView: View {
    @EnvironmentObject var model: Model

    /// Runs an action and closes the panel, like choosing a menu item.
    private func pick(_ action: @escaping () -> Void) -> () -> Void {
        { closePanel(); action() }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            DiskHeader()
            MenuDivider()

            if let scan = model.scan {
                let rows = Array(scan.categories.filter { $0.size_bytes > 0 }.sorted { $0.size_bytes > $1.size_bytes }.prefix(4))
                if !rows.isEmpty {
                    SectionHeader("Cleanable")
                    ForEach(rows) { c in
                        InfoRow(title: c.name, value: oneDecimal(c.size_bytes))
                    }
                    MenuDivider()
                }
                MenuItem(title: scan.safe_bytes > 0 ? "Clean Up \(oneDecimal(scan.safe_bytes))…" : "Nothing to Clean Up",
                         enabled: scan.safe_bytes > 0 && !model.busy, action: pick { Task { await model.cleanSafe() } })
                if scan.review_bytes > 0 {
                    MenuItem(title: "Review \(oneDecimal(scan.review_bytes))…", action: pick { model.openInTerminal("ui") })
                }
            } else {
                StatusRow(text: model.scanning ? "Scanning…" : "No scan yet", spinning: model.scanning)
            }
            if let working = model.working {
                StatusRow(text: working, spinning: true)
            }
            if model.cleaning {
                StatusRow(text: "Cleaning…", spinning: true)
            }
            if let banner = model.banner {
                StatusRow(text: banner.text, spinning: false, warning: banner.isError)
            }

            MenuDivider()
            SectionHeader("Health")
            HealthSection()

            MenuDivider()
            MenuItem(title: model.working ?? "Storage Report", enabled: model.working == nil) { Task { await model.openReport() } }
            MenuItem(title: "Full Health Check…", action: pick { model.openInTerminal("doctor") })
            MenuItem(title: "Undo Last Clean-Up…", action: pick { model.openInTerminal("undo") })
            MenuItem(title: "Settings…", action: pick { Task { await model.openSettings() } })
            MenuToggle(title: "Open at Login", isOn: model.openAtLogin) {
                model.toggleOpenAtLogin()
            }
            MenuToggle(title: "Show Free Space in Menu Bar", isOn: model.showSizeInMenuBar) {
                model.showSizeInMenuBar.toggle()
            }

            MenuDivider()
            MenuItem(title: "Scan Now", shortcut: "⌘R", enabled: !model.busy) { Task { await model.refresh() } }
                .keyboardShortcut("r")
            MenuItem(title: "Quit DustIt", shortcut: "⌘Q") { NSApp.terminate(nil) }
                .keyboardShortcut("q")
            if let updated = model.updated, !model.scanning {
                Text("Updated \(updated, format: .relative(presentation: .named))")
                    .font(.system(size: 11))
                    .foregroundStyle(.tertiary)
                    .padding(.horizontal, 14).padding(.top, 4).padding(.bottom, 2)
            }
        }
        .padding(5)
        .frame(width: 290)
    }
}

// MARK: - Header

private struct DiskHeader: View {
    @EnvironmentObject var model: Model

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .firstTextBaseline) {
                Text("Macintosh HD").font(.system(size: 13, weight: .semibold))
                Spacer()
                Text("\(oneDecimal(model.free)) free")
                    .font(.system(size: 13))
                    .monospacedDigit()
                    .foregroundStyle(.secondary)
            }
            CapacityBar(fraction: model.usedFraction)
            Text("\(oneDecimal(model.used)) of \(oneDecimal(model.total)) used")
                .font(.system(size: 11))
                .monospacedDigit()
                .foregroundStyle(.secondary)
        }
        .padding(.horizontal, 9).padding(.top, 6).padding(.bottom, 4)
    }
}

/// A plain capacity bar like the one in System Settings → Storage.
private struct CapacityBar: View {
    let fraction: Double

    var body: some View {
        GeometryReader { g in
            ZStack(alignment: .leading) {
                RoundedRectangle(cornerRadius: 3).fill(Color.primary.opacity(0.1))
                RoundedRectangle(cornerRadius: 3)
                    .fill(fill)
                    .frame(width: max(6, g.size.width * fraction))
            }
        }
        .frame(height: 6)
    }

    /// Neutral until space actually gets tight.
    private var fill: Color {
        switch fraction {
        case ..<0.85: return Color.primary.opacity(0.55)
        case ..<0.93: return .orange
        default: return .red
        }
    }
}

// MARK: - Health

private struct HealthSection: View {
    @EnvironmentObject var model: Model

    var body: some View {
        if !model.healthLoaded {
            StatusRow(text: "Checking…", spinning: true)
        } else if model.health.isEmpty {
            InfoRow(title: "No problems found", value: "")
        } else {
            ForEach(model.health.prefix(4)) { f in
                InfoRow(title: f.check, value: f.short ?? f.summary,
                        marker: f.severity == "bad" ? .red : .orange)
                    .help(([f.summary] + (f.advice ?? [])).joined(separator: "\n"))
            }
        }
    }
}

// MARK: - Menu building blocks

private struct SectionHeader: View {
    let title: String
    init(_ title: String) { self.title = title }

    var body: some View {
        Text(title)
            .font(.system(size: 11, weight: .semibold))
            .foregroundStyle(.secondary)
            .padding(.horizontal, 9).padding(.top, 3).padding(.bottom, 2)
    }
}

/// A non-interactive line: label on the left, value on the right.
private struct InfoRow: View {
    let title: String
    let value: String
    var marker: Color? = nil

    var body: some View {
        HStack(spacing: 6) {
            if let marker {
                Circle().fill(marker).frame(width: 6, height: 6)
            }
            Text(title).lineLimit(1)
            Spacer(minLength: 12)
            Text(value).lineLimit(1).monospacedDigit().foregroundStyle(.secondary)
        }
        .font(.system(size: 13))
        .padding(.horizontal, 9)
        .frame(height: 22)
    }
}

/// A clickable menu item with the standard accent highlight on hover.
private struct MenuItem: View {
    let title: String
    var shortcut: String? = nil
    var enabled: Bool = true
    let action: () -> Void
    @StateObject private var hover = Hover()

    var body: some View {
        Button(action: action) {
            HStack {
                Text(title)
                Spacer()
                if let shortcut {
                    Text(shortcut).foregroundStyle(highlighted ? Color.white.opacity(0.8) : .secondary)
                }
            }
            .font(.system(size: 13))
            .foregroundStyle(highlighted ? Color.white : (enabled ? Color.primary : Color.secondary.opacity(0.6)))
            .padding(.horizontal, 9)
            .frame(height: 22)
            .contentShape(Rectangle())
            .background(RoundedRectangle(cornerRadius: 4).fill(highlighted ? Color.accentColor : .clear))
        }
        .buttonStyle(.plain)
        .disabled(!enabled)
        .onHover { hover.on = $0 }
    }

    private var highlighted: Bool { hover.on && enabled }
}

private struct MenuToggle: View {
    let title: String
    let isOn: Bool
    let action: () -> Void
    @StateObject private var hover = Hover()

    var body: some View {
        Button(action: action) {
            HStack(spacing: 6) {
                Text(title)
                Spacer()
                if isOn { Image(systemName: "checkmark").font(.system(size: 11, weight: .semibold)) }
            }
            .font(.system(size: 13))
            .foregroundStyle(hover.on ? Color.white : Color.primary)
            .padding(.horizontal, 9)
            .frame(height: 22)
            .contentShape(Rectangle())
            .background(RoundedRectangle(cornerRadius: 4).fill(hover.on ? Color.accentColor : .clear))
        }
        .buttonStyle(.plain)
        .onHover { hover.on = $0 }
    }
}

private struct StatusRow: View {
    let text: String
    let spinning: Bool
    var warning: Bool = false

    var body: some View {
        HStack(spacing: 6) {
            if spinning { ProgressView().controlSize(.mini) }
            if warning { Image(systemName: "exclamationmark.triangle.fill").foregroundStyle(.orange).font(.system(size: 10)) }
            Text(text).fixedSize(horizontal: false, vertical: true)
        }
        .font(.system(size: 12))
        .foregroundStyle(.secondary)
        .padding(.horizontal, 9).padding(.vertical, 3)
    }
}

private struct MenuDivider: View {
    var body: some View {
        Rectangle()
            .fill(Color.primary.opacity(0.1))
            .frame(height: 1)
            .padding(.horizontal, 9).padding(.vertical, 5)
    }
}

/// Closes the menu bar panel (the window that holds it).
@MainActor func closePanel() {
    for w in NSApp.windows where w.isVisible && w.className.contains("MenuBarExtra") {
        w.orderOut(nil)
    }
    NSApp.keyWindow?.orderOut(nil)
}

/// Hover state as a tiny observable object (works without Xcode's SwiftUI macro plugins).
final class Hover: ObservableObject {
    @Published var on = false
}

/// "112.4 GB" — one decimal, Finder-style units.
func oneDecimal(_ bytes: Int64) -> String {
    let units = ["B", "KB", "MB", "GB", "TB"]
    var v = Double(bytes)
    var i = 0
    while v >= 1000 && i < units.count - 1 {
        v /= 1000
        i += 1
    }
    return i == 0 ? "\(bytes) B" : String(format: "%.1f %@", v, units[i])
}
