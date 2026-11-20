import ServiceManagement

/// Open at Login, through macOS's own login items (System Settings → General → Login Items).
enum LoginItem {
    static var isEnabled: Bool { SMAppService.mainApp.status == .enabled }

    /// Turns opening at login on or off. Returns an error message, or nil on success.
    @discardableResult
    static func set(_ on: Bool) -> String? {
        do {
            if on {
                try SMAppService.mainApp.register()
            } else {
                try SMAppService.mainApp.unregister()
            }
            return nil
        } catch {
            return error.localizedDescription
        }
    }

    static var statusText: String {
        switch SMAppService.mainApp.status {
        case .enabled: return "enabled"
        case .notRegistered: return "off"
        case .requiresApproval: return "needs approval in System Settings → General → Login Items"
        case .notFound: return "not found (run DustIt from Applications)"
        @unknown default: return "unknown"
        }
    }
}
