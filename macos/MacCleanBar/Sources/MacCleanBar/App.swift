import AppKit
import SwiftUI

@main
struct MacCleanBarApp: App {
    @StateObject private var model = Model()

    var body: some Scene {
        MenuBarExtra {
            MenuView().environmentObject(model)
        } label: {
            Label(model.title, systemImage: model.low ? "externaldrive.badge.exclamationmark" : "internaldrive")
                .labelStyle(.titleAndIcon)
        }
        .menuBarExtraStyle(.window)
    }
}

@MainActor
final class Model: ObservableObject {
    @Published var free: Int64 = freeSpace()
    @Published var scan: Scan?
    @Published var findings: [Finding] = []
    @Published var busy: String?
    @Published var message: String?
    private var timer: Timer?

    var title: String { human(free) }
    var low: Bool {
        let total = (try? URL(fileURLWithPath: NSHomeDirectory()).resourceValues(forKeys: [.volumeTotalCapacityKey]).volumeTotalCapacity) ?? 0
        return total > 0 && Double(free) < Double(total) * 0.10
    }

    init() {
        timer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.free = freeSpace() }
        }
        Task { await refresh() }
    }

    func refresh() async {
        busy = "Scanning…"
        message = nil
        defer { busy = nil; free = freeSpace() }
        do {
            async let s = CLI.scan()
            async let d = CLI.doctor()
            scan = try await s
            findings = try await d.filter { $0.severity == "bad" || $0.severity == "warn" }
        } catch {
            message = error.localizedDescription
        }
    }

    func cleanSafe() async {
        let alert = NSAlert()
        alert.messageText = "Clean safe categories?"
        alert.informativeText = "Removes caches, logs and other files that apps rebuild automatically (about \(human(scan?.safe_bytes ?? 0))). Review categories are never touched."
        alert.addButton(withTitle: "Clean")
        alert.addButton(withTitle: "Cancel")
        NSApp.activate(ignoringOtherApps: true)
        guard alert.runModal() == .alertFirstButtonReturn else { return }
        busy = "Cleaning…"
        let before = freeSpace()
        do {
            _ = try await CLI.cleanSafe()
            message = "Freed \(human(max(freeSpace() - before, 0)))."
        } catch {
            message = error.localizedDescription
        }
        busy = nil
        await refresh()
    }

    /// Opens Terminal running a macclean command (for the interactive picker).
    func openInTerminal(_ args: String) {
        guard let path = CLI.path else { return }
        let script = "tell application \"Terminal\" to do script \"'\(path)' \(args)\"\ntell application \"Terminal\" to activate"
        NSAppleScript(source: script)?.executeAndReturnError(nil)
    }

    func openReport() async {
        busy = "Building report…"
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("macclean-report.html").path
        do { _ = try await CLI.run(["report", "--no-doctor", "-o", file, "--open"]) } catch { message = error.localizedDescription }
        busy = nil
    }
}

struct MenuView: View {
    @EnvironmentObject var model: Model

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                VStack(alignment: .leading) {
                    Text("\(human(model.free)) free").font(.title2.bold())
                    if let s = model.scan {
                        Text("\(human(s.safe_bytes)) safe to clean · \(human(s.review_bytes)) to review")
                            .font(.callout).foregroundStyle(.secondary)
                    }
                }
                Spacer()
                if model.busy != nil { ProgressView().controlSize(.small) }
            }

            if let busy = model.busy {
                Text(busy).font(.callout).foregroundStyle(.secondary)
            }
            if let msg = model.message {
                Text(msg).font(.callout).foregroundStyle(.orange).fixedSize(horizontal: false, vertical: true)
            }

            if let s = model.scan {
                let top = s.categories.filter { $0.size_bytes > 0 }.sorted { $0.size_bytes > $1.size_bytes }.prefix(5)
                if !top.isEmpty {
                    Divider()
                    ForEach(Array(top)) { c in
                        HStack {
                            Circle().fill(c.risk == "safe" ? .green : .orange).frame(width: 7, height: 7)
                            Text(c.name).lineLimit(1)
                            Spacer()
                            Text(human(c.size_bytes)).monospacedDigit().foregroundStyle(.secondary)
                        }.font(.callout)
                    }
                }
            }

            if !model.findings.isEmpty {
                Divider()
                Text("Slowing your Mac").font(.headline)
                ForEach(model.findings) { f in
                    VStack(alignment: .leading, spacing: 2) {
                        Text("\(f.severity == "bad" ? "✗" : "!") \(f.check)").font(.callout.bold())
                        Text(f.summary).font(.caption).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                    }
                }
            }

            Divider()
            Button("Clean safe categories…") { Task { await model.cleanSafe() } }
                .disabled(model.busy != nil || (model.scan?.safe_bytes ?? 0) == 0)
            Button("Choose what to clean (opens Terminal)") { model.openInTerminal("ui") }
            Button("Open report") { Task { await model.openReport() } }.disabled(model.busy != nil)
            Button("Rescan") { Task { await model.refresh() } }.disabled(model.busy != nil)
            Divider()
            Button("Quit macclean bar") { NSApp.terminate(nil) }
        }
        .buttonStyle(.borderless)
        .padding(14)
        .frame(width: 320)
    }
}
