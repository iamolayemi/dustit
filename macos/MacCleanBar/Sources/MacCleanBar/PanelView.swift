import SwiftUI

struct PanelView: View {
    @EnvironmentObject var model: Model

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Header()
                .padding(16)
            if let banner = model.banner {
                BannerView(banner: banner) { model.banner = nil }
                    .padding(.horizontal, 12).padding(.bottom, 10)
            }
            CleanCard()
                .padding(.horizontal, 12)
            if let scan = model.scan {
                Section(title: "Where the space is") { CategoryList(scan: scan) }
            }
            Section(title: "Health") { HealthList() }
            Divider().padding(.top, 6)
            Footer()
                .padding(.horizontal, 10).padding(.vertical, 8)
        }
        .frame(width: 340)
    }
}

// MARK: - Header

private struct Header: View {
    @EnvironmentObject var model: Model

    var body: some View {
        HStack(spacing: 14) {
            UsageRing(fraction: model.usedFraction, tint: ringTint)
                .frame(width: 46, height: 46)
            VStack(alignment: .leading, spacing: 2) {
                Text("\(human(model.free)) free")
                    .font(.system(size: 20, weight: .semibold, design: .rounded))
                    .monospacedDigit()
                Text("of \(human(model.total)) · \(Int((model.usedFraction * 100).rounded()))% used")
                    .font(.callout)
                    .foregroundStyle(.secondary)
            }
            Spacer(minLength: 0)
        }
    }

    private var ringTint: Color {
        switch model.usedFraction {
        case ..<0.75: return .green
        case ..<0.9: return .orange
        default: return .red
        }
    }
}

private struct UsageRing: View {
    let fraction: Double
    let tint: Color

    var body: some View {
        ZStack {
            Circle().stroke(Color.primary.opacity(0.1), lineWidth: 6)
            Circle()
                .trim(from: 0, to: fraction)
                .stroke(tint.gradient, style: StrokeStyle(lineWidth: 6, lineCap: .round))
                .rotationEffect(.degrees(-90))
                .animation(.easeOut(duration: 0.6), value: fraction)
            Image(systemName: "internaldrive.fill")
                .font(.system(size: 14))
                .foregroundStyle(.secondary)
        }
    }
}

// MARK: - Clean

private struct CleanCard: View {
    @EnvironmentObject var model: Model

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Button {
                Task { await model.cleanSafe() }
            } label: {
                HStack {
                    if model.cleaning {
                        ProgressView().controlSize(.small).tint(.white)
                        Text("Cleaning…")
                    } else {
                        Image(systemName: "sparkles")
                        Text(cleanTitle)
                    }
                }
                .frame(maxWidth: .infinity)
                .padding(.vertical, 3)
            }
            .buttonStyle(.borderedProminent)
            .controlSize(.large)
            .disabled(model.busy || (model.scan?.safe_bytes ?? 0) == 0)

            if let review = model.scan?.review_bytes, review > 0 {
                Button {
                    model.openInTerminal("ui")
                } label: {
                    HStack(spacing: 4) {
                        Text("\(human(review)) more needs a look")
                        Image(systemName: "chevron.right").font(.caption2.weight(.semibold))
                    }
                    .font(.callout)
                    .frame(maxWidth: .infinity)
                }
                .buttonStyle(.plain)
                .foregroundStyle(.secondary)
                .help("Opens the interactive picker in Terminal")
            }
        }
    }

    private var cleanTitle: String {
        guard let scan = model.scan else { return model.scanning ? "Scanning…" : "Clean" }
        return scan.safe_bytes > 0 ? "Clean \(human(scan.safe_bytes))" : "Nothing safe to clean"
    }
}

// MARK: - Categories

private struct CategoryList: View {
    let scan: Scan

    var body: some View {
        let top = scan.categories.filter { $0.size_bytes > 0 }.sorted { $0.size_bytes > $1.size_bytes }.prefix(5)
        let maxSize = top.first?.size_bytes ?? 1
        VStack(spacing: 2) {
            ForEach(Array(top)) { c in
                HoverRow {
                    HStack(spacing: 10) {
                        Circle().fill(c.risk == "safe" ? Color.green : Color.orange).frame(width: 7, height: 7)
                        VStack(alignment: .leading, spacing: 4) {
                            HStack {
                                Text(c.name).lineLimit(1)
                                Spacer()
                                Text(human(c.size_bytes)).monospacedDigit().foregroundStyle(.secondary)
                            }
                            .font(.callout)
                            GeometryReader { g in
                                Capsule().fill(Color.primary.opacity(0.08))
                                    .overlay(alignment: .leading) {
                                        Capsule()
                                            .fill((c.risk == "safe" ? Color.green : Color.orange).opacity(0.7))
                                            .frame(width: max(3, g.size.width * Double(c.size_bytes) / Double(maxSize)))
                                    }
                            }
                            .frame(height: 3)
                        }
                    }
                    .help(c.risk == "safe" ? "Safe: rebuilt automatically" : "Review: may contain things you want")
                }
            }
        }
    }
}

// MARK: - Health

private struct HealthList: View {
    @EnvironmentObject var model: Model

    var body: some View {
        if !model.healthLoaded {
            HStack(spacing: 8) {
                ProgressView().controlSize(.small)
                Text("Checking…").foregroundStyle(.secondary)
            }
            .font(.callout)
            .padding(.horizontal, 8).padding(.vertical, 4)
        } else if model.health.isEmpty {
            Label("Nothing is slowing your Mac down", systemImage: "checkmark.circle.fill")
                .font(.callout)
                .foregroundStyle(.green)
                .padding(.horizontal, 8).padding(.vertical, 4)
        } else {
            VStack(spacing: 2) {
                ForEach(model.health) { f in
                    HoverRow {
                        HStack(alignment: .top, spacing: 10) {
                            Image(systemName: symbol(for: f.check))
                                .font(.system(size: 13, weight: .medium))
                                .foregroundStyle(color(for: f.severity))
                                .frame(width: 18)
                            VStack(alignment: .leading, spacing: 1) {
                                Text(f.check).font(.callout.weight(.medium))
                                Text(f.summary)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                    .lineLimit(2)
                                    .fixedSize(horizontal: false, vertical: true)
                            }
                            Spacer(minLength: 0)
                        }
                        .help((f.advice ?? []).joined(separator: "\n"))
                    }
                }
            }
        }
    }

    private func symbol(for check: String) -> String {
        switch check {
        case "CPU load": return "cpu"
        case "Memory": return "memorychip"
        case "Disk space": return "internaldrive"
        case "Battery": return "battery.25percent"
        case "Temperature": return "thermometer.medium"
        case "Spotlight": return "magnifyingglass"
        case "Browsers": return "safari"
        case "Docker": return "shippingbox"
        case "Startup items": return "power"
        case "macOS updates": return "arrow.down.circle"
        case "Uptime": return "clock"
        default: return "stethoscope"
        }
    }

    private func color(for severity: String) -> Color {
        switch severity {
        case "bad": return .red
        case "warn": return .orange
        default: return .blue
        }
    }
}

// MARK: - Footer

private struct Footer: View {
    @EnvironmentObject var model: Model

    var body: some View {
        HStack(spacing: 2) {
            IconButton(symbol: "arrow.clockwise", help: "Scan again") { Task { await model.refresh() } }
                .disabled(model.busy)
            IconButton(symbol: "doc.richtext", help: "Open the HTML report") { Task { await model.openReport() } }
            IconButton(symbol: "checklist", help: "Choose what to clean (opens Terminal)") { model.openInTerminal("ui") }
            Spacer()
            Group {
                if model.scanning {
                    HStack(spacing: 6) {
                        ProgressView().controlSize(.mini)
                        Text("Scanning…")
                    }
                } else if let updated = model.updated {
                    Text("Updated \(updated, format: .relative(presentation: .named))")
                }
            }
            .font(.caption)
            .foregroundStyle(.secondary)
            Menu {
                Button("Doctor in Terminal") { model.openInTerminal("doctor") }
                Button("Edit settings…") { model.openInTerminal("config edit") }
                Button("Undo last clean") { model.openInTerminal("undo") }
                Divider()
                Toggle("Show free space in the menu bar", isOn: Binding(get: { model.showSizeInMenuBar }, set: { model.showSizeInMenuBar = $0 }))
                Divider()
                Button("Quit macclean bar") { NSApp.terminate(nil) }
            } label: {
                Image(systemName: "ellipsis.circle")
            }
            .menuStyle(.borderlessButton)
            .menuIndicator(.hidden)
            .fixedSize()
            .padding(.leading, 6)
        }
    }
}

// MARK: - Building blocks

private struct Section<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(title.uppercased())
                .font(.caption2.weight(.semibold))
                .foregroundStyle(.secondary)
                .tracking(0.6)
                .padding(.horizontal, 16)
            content.padding(.horizontal, 8)
        }
        .padding(.top, 16)
    }
}

/// Hover state as a tiny observable object (works without Xcode's SwiftUI macro plugins).
final class Hover: ObservableObject {
    @Published var on = false
}

private struct HoverRow<Content: View>: View {
    @ViewBuilder let content: Content
    @StateObject private var hover = Hover()

    var body: some View {
        content
            .padding(.horizontal, 8).padding(.vertical, 6)
            .background(RoundedRectangle(cornerRadius: 7).fill(Color.primary.opacity(hover.on ? 0.07 : 0)))
            .onHover { hover.on = $0 }
    }
}

private struct IconButton: View {
    let symbol: String
    let help: String
    let action: () -> Void
    @StateObject private var hover = Hover()

    var body: some View {
        Button(action: action) {
            Image(systemName: symbol)
                .font(.system(size: 13, weight: .medium))
                .frame(width: 28, height: 24)
                .background(RoundedRectangle(cornerRadius: 6).fill(Color.primary.opacity(hover.on ? 0.08 : 0)))
        }
        .buttonStyle(.plain)
        .foregroundStyle(.secondary)
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
                .foregroundStyle(banner.isError ? .orange : .green)
            Text(banner.text)
                .font(.caption)
                .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
            Button(action: dismiss) { Image(systemName: "xmark").font(.caption2.weight(.bold)) }
                .buttonStyle(.plain)
                .foregroundStyle(.secondary)
        }
        .padding(10)
        .background(RoundedRectangle(cornerRadius: 8).fill((banner.isError ? Color.orange : Color.green).opacity(0.12)))
    }
}
