import Foundation

/// Talks to the `macclean` command-line tool, which does all scanning and cleaning.
enum CLI {
    /// Finds the macclean binary in the usual install locations.
    static let path: String? = {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        let candidates = [
            ProcessInfo.processInfo.environment["MACCLEAN_BIN"],
            "/opt/homebrew/bin/macclean", "/usr/local/bin/macclean",
            "\(home)/.local/bin/macclean", "\(home)/go/bin/macclean",
        ].compactMap { $0 }
        return candidates.first { FileManager.default.isExecutableFile(atPath: $0) }
    }()

    struct Failure: Error, LocalizedError {
        let message: String
        var errorDescription: String? { message }
    }

    /// Runs macclean with arguments and returns stdout.
    static func run(_ args: [String]) async throws -> Data {
        guard let path else {
            throw Failure(message: "macclean isn't installed. Install it with: brew install iamolayemi/tap/macclean")
        }
        return try await withCheckedThrowingContinuation { cont in
            let p = Process()
            p.executableURL = URL(fileURLWithPath: path)
            p.arguments = args + ["--no-color"]
            let out = Pipe(), err = Pipe()
            p.standardOutput = out
            p.standardError = err
            p.standardInput = FileHandle.nullDevice
            p.terminationHandler = { proc in
                let data = out.fileHandleForReading.readDataToEndOfFile()
                if proc.terminationStatus == 0 {
                    cont.resume(returning: data)
                } else {
                    let msg = String(data: err.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
                    cont.resume(throwing: Failure(message: msg.isEmpty ? "macclean exited with \(proc.terminationStatus)" : msg))
                }
            }
            do { try p.run() } catch { cont.resume(throwing: error) }
        }
    }

    static func scan() async throws -> Scan {
        try JSONDecoder().decode(Scan.self, from: try await run(["scan", "--json"]))
    }

    static func doctor() async throws -> [Finding] {
        try JSONDecoder().decode([Finding].self, from: try await run(["doctor", "--json"]))
    }

    /// Cleans the safe categories (never review ones) without prompts.
    static func cleanSafe() async throws -> String {
        String(data: try await run(["clean", "--yes"]), encoding: .utf8) ?? ""
    }
}

struct Scan: Decodable {
    let free_bytes: Int64
    let safe_bytes: Int64
    let review_bytes: Int64
    let categories: [Category]

    struct Category: Decodable, Identifiable {
        let id: String
        let name: String
        let risk: String
        let size_bytes: Int64
    }
}

struct Finding: Decodable, Identifiable {
    let check: String
    let severity: String
    let summary: String
    var short: String? = nil
    let advice: [String]?
    var id: String { check }
}

func human(_ bytes: Int64) -> String {
    ByteCountFormatter.string(fromByteCount: bytes, countStyle: .file)
}

/// Free space on the startup volume, read directly (cheap, no CLI call).
func freeSpace() -> Int64 {
    let url = URL(fileURLWithPath: NSHomeDirectory())
    let v = try? url.resourceValues(forKeys: [.volumeAvailableCapacityForImportantUsageKey])
    return v?.volumeAvailableCapacityForImportantUsage ?? 0
}
