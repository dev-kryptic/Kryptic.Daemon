import AppKit
import Darwin
import Foundation

enum SingleInstanceGuard {
    private static let lockPath = (NSTemporaryDirectory() as NSString)
        .appendingPathComponent("dev.kryptic.daemon.lock")
    private static var lockFD: Int32 = -1

    /// Takes the single-instance lock. If an older Kryptic is still running
    /// (typical after a pkg/dmg upgrade), that instance is asked to quit so
    /// this process can continue. The Keychain session is not cleared.
    static func acquire() -> Bool {
        if tryLock() { return true }

        for app in NSRunningApplication.runningApplications(withBundleIdentifier: "dev.kryptic.daemon")
            where app != NSRunningApplication.current {
            app.terminate()
        }

        for _ in 0..<30 {
            usleep(100_000)
            if tryLock() { return true }
        }

        for app in NSRunningApplication.runningApplications(withBundleIdentifier: "dev.kryptic.daemon")
            where app != NSRunningApplication.current {
            app.forceTerminate()
        }

        for _ in 0..<20 {
            usleep(100_000)
            if tryLock() { return true }
        }

        return tryLock()
    }

    private static func tryLock() -> Bool {
        if lockFD >= 0 {
            flock(lockFD, LOCK_UN)
            close(lockFD)
            lockFD = -1
        }
        lockFD = open(lockPath, O_CREAT | O_RDWR, 0o644)
        guard lockFD >= 0 else { return true }
        if flock(lockFD, LOCK_EX | LOCK_NB) == 0 {
            return true
        }
        close(lockFD)
        lockFD = -1
        return false
    }
}
