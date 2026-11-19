import SwiftUI

/// macclean's accent: a calm teal.
let brand = Color(red: 0.13, green: 0.60, blue: 0.53)

struct PanelView: View {
    @EnvironmentObject var model: Model

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Header()
            if let banner = model.banner {
                BannerView(banner: banner) { model.banner = nil }
            }
            CleanBlock()
            if let scan = model.scan, scan.categories.contains(where: { $0.size_bytes > 0 }) {
                Card(title: "Where the space is") { CategoryRows(scan: scan) }
            }
            Card(title: "Health") { HealthRows() }
            Footer()
        }
        .padding(14)
        .frame(width: 330)
    }
}

// MARK: - Header

private struct Header: View {
    @EnvironmentObject var model: Model

    var body: some View {
        HStack(spacing: 12) {
            Ring(fraction: model.usedFraction, tint: tint)
                .frame(width: 40, height: 40)
            VStack(alignment: .leading, spacing: 1) {
                Text("\(oneDecimal(model.free)) available")
                    .font(.system(size: 17, weight: .semibold, design: .rounded))
                    .monospacedDigit()
                Text("of \(oneDecimal(model.total)) on Macintosh HD")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer(minLength: 0)
            if model.scanning {
                ProgressView().controlSize(.small).help("Scanning…")
            }
        }
    }

    private var tint: Color {
        switch model.usedFraction {
        case ..<0.8: return brand
        case ..<0.9: return .orange
        default: return .red
        }
    }
}

private struct Ring: View {
    let fraction: Double
    let tint: Color

    var body: some View {
        ZStack {
            Circle().stroke(Color.primary.opacity(0.09), lineWidth: 4)
            Circle()
                .trim(from: 0, to: fraction)
                .stroke(tint, style: StrokeStyle(lineWidth: 4, lineCap: .round))
                .rotationEffect(.degrees(-90))
                .animation(.easeOut(duration: 0.5), value: fraction)
            Text("\(Int((fraction * 100).rounded()))%")
                .font(.system(size: 10, weight: .semibold, design: .rounded))
                .monospacedDigit()
                .foregroundStyle(.secondary)
        }
    }
}

// MARK: - Clean

private struct CleanBlock: View {
    @EnvironmentObject var model: Model

    var body: some View {
        VStack(spacing: 6) {
            Button {
                Task { await model.cleanSafe() }
            } label: {
                HStack(spacing: 6) {
                    if model.cleaning {
                        ProgressView().controlSize(.small)
                        Text("Cleaning…")
                    } else {
                        Text(title)
                    }
                }
                .font(.system(size: 13, weight: .semibold))
                .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent)
            .tint(brand)
            .controlSize(.large)
            .disabled(model.busy || (model.scan?.safe_bytes ?? 0) == 0)

            if let review = model.scan?.review_bytes, review > 0 {
                Button { model.openInTerminal("ui") } label: {
                    Text("\(oneDecimal(review)) more to review →")
                        .font(.caption)
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.plain)
                .foregroundStyle(.secondary)
                .help("Choose what to remove in the interactive picker (opens Terminal)")
            }
        }
    }

    private var title: String {
        guard let scan = model.scan else { return model.scanning ? "Scanning…" : "Clean" }
        return scan.safe_bytes > 0 ? "Clean \(oneDecimal(scan.safe_bytes)) safely" : "Nothing safe to clean right now"
    }
}

// MARK: - Rows

private struct CategoryRows: View {
    let scan: Scan

    var body: some View {
        let top = Array(scan.categories.filter { $0.size_bytes > 0 }.sorted { $0.size_bytes > $1.size_bytes }.prefix(4))
        ForEach(Array(top.enumerated()), id: \.element.id) { i, c in
            Row(divider: i > 0) {
                IconTile(symbol: categorySymbol(c.id), tint: c.risk == "safe" ? brand : .orange)
                VStack(alignment: .leading, spacing: 1) {
                    Text(c.name).font(.callout).lineLimit(1)
                    Text(c.risk == "safe" ? "Rebuilt automatically" : "Worth a look first")
                        .font(.caption2).foregroundStyle(.secondary)
                }
                Spacer(minLength: 8)
                Text(oneDecimal(c.size_bytes))
                    .font(.callout).monospacedDigit().foregroundStyle(.secondary)
            }
        }
    }
}

private struct HealthRows: View {
    @EnvironmentObject var model: Model

    var body: some View {
        if !model.healthLoaded {
            Row(divider: false) {
                ProgressView().controlSize(.small).frame(width: 26)
                Text("Checking your Mac…").font(.callout).foregroundStyle(.secondary)
                Spacer()
            }
        } else if model.health.isEmpty {
            Row(divider: false) {
                IconTile(symbol: "checkmark", tint: brand)
                Text("Nothing is slowing your Mac down").font(.callout)
                Spacer()
            }
        } else {
            let shown = Array(model.health.prefix(3))
            ForEach(Array(shown.enumerated()), id: \.element.id) { i, f in
                Row(divider: i > 0) {
                    IconTile(symbol: healthSymbol(f.check), tint: f.severity == "bad" ? .red : .orange)
                    VStack(alignment: .leading, spacing: 1) {
                        Text(f.check).font(.callout)
                        Text(f.short ?? f.summary).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
                    }
                    Spacer(minLength: 0)
                }
                .help(([f.summary] + (f.advice ?? [])).joined(separator: "\n"))
            }
            if model.health.count > shown.count {
                Row(divider: true) {
                    Button { model.openInTerminal("doctor") } label: {
                        Text("\(model.health.count - shown.count) more — open the full check →").font(.caption)
                    }
                    .buttonStyle(.plain)
                    .foregroundStyle(.secondary)
                    Spacer()
                }
            }
        }
    }
}

private func categorySymbol(_ id: String) -> String {
    switch id {
    case "user-caches", "electron-caches", "dot-cache": return "tray.full"
    case "user-logs", "gradle-daemon-logs": return "doc.text"
    case "trash": return "trash"
    case "old-downloads", "installers": return "arrow.down.circle"
    case "stale-deps", "npm-cache", "yarn-cache", "pnpm-store", "pnpm-old-stores", "bun-cache", "composer-cache",
         "cargo-cache", "gradle-cache", "maven-repo", "cocoapods-specs", "go-modcache": return "shippingbox"
    case "old-versions", "jetbrains-old": return "square.stack.3d.up"
    case "old-files", "large-videos": return "clock.arrow.circlepath"
    case "duplicates": return "doc.on.doc"
    case "screenshots": return "camera.viewfinder"
    case "docker", "dev-vms", "minikube-cache": return "cube"
    case "xcode-derived", "xcode-device-support", "simulator-caches", "simulator-devices", "xcode-archives": return "hammer"
    case "ai-models", "ai-tool-data": return "sparkles"
    case "homebrew": return "mug"
    case "git-gc": return "arrow.triangle.branch"
    case "icloud-evict": return "icloud"
    default: return "folder"
    }
}

private func healthSymbol(_ check: String) -> String {
    switch check {
    case "CPU load": return "cpu"
    case "Memory": return "memorychip"
    case "Disk space": return "internaldrive"
    case "Battery": return "battery.25percent"
    case "Temperature": return "thermometer.medium"
    case "Spotlight": return "magnifyingglass"
    case "Browsers": return "globe"
    case "Docker": return "cube"
    case "Startup items": return "power"
    case "macOS updates": return "arrow.down.circle"
    default: return "stethoscope"
    }
}

// MARK: - Footer

private struct Footer: View {
    @EnvironmentObject var model: Model

    var body: some View {
        HStack(spacing: 0) {
            IconButton(symbol: "arrow.clockwise", help: "Scan again") { Task { await model.refresh() } }
                .disabled(model.busy)
            IconButton(symbol: "chart.pie", help: "Open the storage report") { Task { await model.openReport() } }
            IconButton(symbol: "checklist", help: "Choose what to clean (opens Terminal)") { model.openInTerminal("ui") }
            Spacer()
            Group {
                if model.scanning {
                    Text("Scanning…")
                } else if let updated = model.updated {
                    Text("Updated \(updated, format: .relative(presentation: .named))")
                }
            }
            .font(.caption2)
            .foregroundStyle(.tertiary)
            Menu {
                Button("Full health check") { model.openInTerminal("doctor") }
                Button("Undo last clean") { model.openInTerminal("undo") }
                Button("Settings…") { model.openInTerminal("config edit") }
                Divider()
                Toggle("Show free space in the menu bar", isOn: Binding(get: { model.showSizeInMenuBar }, set: { model.showSizeInMenuBar = $0 }))
                Divider()
                Button("Quit") { NSApp.terminate(nil) }
            } label: {
                Image(systemName: "ellipsis")
            }
            .menuStyle(.borderlessButton)
            .menuIndicator(.hidden)
            .fixedSize()
            .padding(.leading, 8)
        }
        .foregroundStyle(.secondary)
    }
}

// MARK: - Building blocks

/// A titled, rounded group of rows, like System Settings.
private struct Card<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(title)
                .font(.caption.weight(.medium))
                .foregroundStyle(.secondary)
                .padding(.leading, 4)
            VStack(spacing: 0) { content }
                .background(RoundedRectangle(cornerRadius: 10, style: .continuous).fill(Color.primary.opacity(0.045)))
                .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).strokeBorder(Color.primary.opacity(0.06)))
        }
    }
}

private struct Row<Content: View>: View {
    let divider: Bool
    @ViewBuilder let content: Content
    @StateObject private var hover = Hover()

    var body: some View {
        VStack(spacing: 0) {
            if divider { Divider().padding(.leading, 46) }
            HStack(spacing: 10) { content }
                .padding(.horizontal, 10).padding(.vertical, 7)
                .background(Color.primary.opacity(hover.on ? 0.04 : 0))
                .onHover { hover.on = $0 }
        }
    }
}

private struct IconTile: View {
    let symbol: String
    let tint: Color

    var body: some View {
        Image(systemName: symbol)
            .font(.system(size: 12, weight: .semibold))
            .foregroundStyle(tint)
            .frame(width: 26, height: 26)
            .background(RoundedRectangle(cornerRadius: 7, style: .continuous).fill(tint.opacity(0.14)))
    }
}

/// Hover state as a tiny observable object (works without Xcode's SwiftUI macro plugins).
final class Hover: ObservableObject {
    @Published var on = false
}

private struct IconButton: View {
    let symbol: String
    let help: String
    let action: () -> Void
    @StateObject private var hover = Hover()

    var body: some View {
        Button(action: action) {
            Image(systemName: symbol)
                .font(.system(size: 12, weight: .medium))
                .frame(width: 28, height: 24)
                .background(RoundedRectangle(cornerRadius: 6).fill(Color.primary.opacity(hover.on ? 0.08 : 0)))
        }
        .buttonStyle(.plain)
        .help(help)
        .onHover { hover.on = $0 }
    }
}

private struct BannerView: View {
    let banner: Model.Banner
    let dismiss: () -> Void

    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: banner.isError ? "exclamationmark.triangle.fill" : "checkmark.circle.fill")
                .foregroundStyle(banner.isError ? Color.orange : brand)
            Text(banner.text).font(.caption).fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
            Button(action: dismiss) { Image(systemName: "xmark").font(.caption2.weight(.bold)) }
                .buttonStyle(.plain).foregroundStyle(.secondary)
        }
        .padding(10)
        .background(RoundedRectangle(cornerRadius: 9, style: .continuous).fill((banner.isError ? Color.orange : brand).opacity(0.12)))
    }
}

/// "112.5 GB" — one decimal, Finder-style units.
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
