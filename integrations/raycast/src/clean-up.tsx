import { Alert, confirmAlert, showHUD, showToast, Toast } from "@raycast/api";
import { human, run, runJSON, Scan } from "./dustit";

export default async function CleanUp() {
  const toast = await showToast({ style: Toast.Style.Animated, title: "Scanning…" });
  try {
    const scan = await runJSON<Scan>(["scan"]);
    await toast.hide();
    if (scan.safe_bytes === 0) {
      await showHUD("Nothing safe to clean right now");
      return;
    }
    const ok = await confirmAlert({
      title: `Clean ${human(scan.safe_bytes)}?`,
      message: "Removes caches, logs and other files apps rebuild by themselves. Caches of open apps and anything that needs review are left alone.",
      primaryAction: { title: "Clean", style: Alert.ActionStyle.Destructive },
    });
    if (!ok) return;
    await showToast({ style: Toast.Style.Animated, title: "Cleaning…" });
    await run(["clean", "--yes"]);
    await showHUD(`DustIt freed about ${human(scan.safe_bytes)}`);
  } catch (e) {
    await showToast({ style: Toast.Style.Failure, title: "DustIt couldn't clean", message: String((e as Error).message) });
  }
}
