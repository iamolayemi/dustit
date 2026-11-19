import AppKit
import Foundation

@MainActor
final class Model: ObservableObject {
    @Published var free: Int64 = freeSpace()
    @Published var total: Int64 = totalSpace()
    @Published var scan: Scan?
    @Published var health: [Finding] = []
    @Published var healthLoaded = false
    @Published var scanning = false
    @Published var checking = false
    @Published var cleaning = false
    @Published var banner: Banner?
    @Published var updated: Date?
    /// Icon-only keeps the item narrow, so a crowded (notched) menu bar is less likely to hide it.
    @Published var showSizeInMenuBar: Bool = UserDefaults.standard.object(forKey: "showSize") as? Bool ?? true {
        didSet { UserDefaults.standard.set(showSizeInMenuBar, forKey: "showSize") }
    }
    private var timers: [Timer] = []

    struct Banner: Equatable {
        let text: String
        let isError: Bool
    }

    var used: Int64 { max(total - free, 0) }
    var usedFraction: Double { total > 0 ? Double(used) / Double(total) : 0 }
    var isLow: Bool { total > 0 && Double(free) < Double(total) * 0.10 }
    var busy: Bool { scanning || cleaning }

    /// Sample data for snapshots and previews; makes no CLI calls.
    init(sample: Bool) {
        free = 112_410_000_000
        total = 494_380_000_000
        scan = Scan(free_bytes: free, safe_bytes: 1_820_000_000, review_bytes: 4_880_000_000, categories: [
            .init(id: "old-downloads", name: "Downloads older than 90 days", risk: "review", size_bytes: 4_120_000_000),
            .init(id: "user-caches", name: "App caches", risk: "safe", size_bytes: 1_260_000_000),
            .init(id: "dot-cache", name: "~/.cache tool caches", risk: "safe", size_bytes: 468_400_000),
            .init(id: "old-files", name: "Large files untouched for a year", risk: "review", size_bytes: 421_900_000),
            .init(id: "go-modcache", name: "Go module cache", risk: "review", size_bytes: 256_500_000),
        ])
        health = [
            Finding(check: "Battery", severity: "bad", summary: "check battery condition, 69% of original capacity, 807 charge cycles", short: "69% capacity · 807 cycles", advice: nil),
            Finding(check: "CPU load", severity: "warn", summary: "13.3 tasks waiting on 8 cores (5-min average)", short: "2× more work than it can run", advice: nil),
            Finding(check: "Memory", severity: "warn", summary: "17.2 GB RAM, 10.8 GB swap in use — memory pressure is elevated", short: "10.8 GB swapped to disk", advice: nil),
        ]
        healthLoaded = true
        updated = Date().addingTimeInterval(-180)
    }

    init() {
        // Free space is cheap to read; a full scan is not, so it runs on open (if stale) and every 6 hours.
        timers.append(Timer.scheduledTimer(withTimeInterval: 30, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.free = freeSpace() }
        })
        timers.append(Timer.scheduledTimer(withTimeInterval: 6 * 3600, repeats: true) { [weak self] _ in
            Task { @MainActor in await self?.refresh() }
        })
        Task { await refresh() }
    }

    func panelOpened() {
        free = freeSpace()
        if let updated, Date().timeIntervalSince(updated) < 15 * 60 { return }
        Task { await refresh() }
    }

    /// Health first (a few seconds), then the slower scan. Old results stay visible meanwhile.
    func refresh() async {
        guard !scanning else { return }
        scanning = true
        checking = true
        defer { scanning = false; free = freeSpace() }
        do {
            health = try await CLI.doctor().filter { $0.severity == "bad" || $0.severity == "warn" }
            healthLoaded = true
        } catch {
            banner = Banner(text: error.localizedDescription, isError: true)
        }
        checking = false
        do {
            scan = try await CLI.scan()
            updated = Date()
        } catch {
            banner = Banner(text: error.localizedDescription, isError: true)
        }
    }

    func cleanSafe() async {
        let alert = NSAlert()
        alert.messageText = "Clean \(human(scan?.safe_bytes ?? 0))?"
        alert.informativeText = "Removes caches, logs and other files apps rebuild by themselves. Caches of open apps and everything that needs review are left alone."
        alert.addButton(withTitle: "Clean")
        alert.addButton(withTitle: "Cancel")
        NSApp.activate(ignoringOtherApps: true)
        guard alert.runModal() == .alertFirstButtonReturn else { return }
        cleaning = true
        let before = freeSpace()
        do {
            _ = try await CLI.cleanSafe()
            banner = Banner(text: "Freed \(human(max(freeSpace() - before, 0))). Undo trashed items with: macclean undo", isError: false)
        } catch {
            banner = Banner(text: error.localizedDescription, isError: true)
        }
        cleaning = false
        await refresh()
    }

    /// Opens Terminal running a macclean command (used for the interactive picker).
    func openInTerminal(_ args: String) {
        guard let path = CLI.path else { return }
        let script = "tell application \"Terminal\" to do script \"'\(path)' \(args)\"\ntell application \"Terminal\" to activate"
        NSAppleScript(source: script)?.executeAndReturnError(nil)
    }

    func openReport() async {
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("macclean-report.html").path
        do { _ = try await CLI.run(["report", "--no-doctor", "-o", file, "--open"]) } catch {
            banner = Banner(text: error.localizedDescription, isError: true)
        }
    }
}

func totalSpace() -> Int64 {
    let v = try? URL(fileURLWithPath: NSHomeDirectory()).resourceValues(forKeys: [.volumeTotalCapacityKey])
    return Int64(v?.volumeTotalCapacity ?? 0)
}

/// "112 GB" — whole numbers for the menu bar.
func shortSize(_ bytes: Int64) -> String {
    let gb = Double(bytes) / 1e9
    return gb >= 10 ? "\(Int(gb.rounded())) GB" : String(format: "%.1f GB", gb)
}
