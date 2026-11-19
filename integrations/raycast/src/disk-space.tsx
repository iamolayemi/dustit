import { Action, ActionPanel, Color, Icon, List, showToast, Toast } from "@raycast/api";
import { useEffect, useState } from "react";
import { Group, human, run, runJSON, Scan } from "./dustit";

export default function DiskSpace() {
  const [groups, setGroups] = useState<Group[]>();
  const [scan, setScan] = useState<Scan>();
  const [error, setError] = useState<string>();

  useEffect(() => {
    runJSON<{ groups: Group[] }>(["storage"]).then((r) => setGroups(r.groups)).catch((e) => setError(String(e.message)));
    runJSON<Scan>(["scan"]).then(setScan).catch((e) => setError(String(e.message)));
  }, []);

  if (error) {
    return (
      <List>
        <List.EmptyView icon={Icon.Warning} title="DustIt couldn't run" description={error} />
      </List>
    );
  }

  const cleanable = (scan?.categories ?? []).filter((c) => c.size_bytes > 0).sort((a, b) => b.size_bytes - a.size_bytes);
  return (
    <List isLoading={!groups || !scan} searchBarPlaceholder="Filter groups and categories">
      {scan && (
        <List.Section title={`${human(scan.free_bytes)} free · ${human(scan.safe_bytes)} safe to clean`}>
          {cleanable.map((c) => (
            <List.Item
              key={c.id}
              title={c.name}
              subtitle={c.description}
              icon={{ source: Icon.Circle, tintColor: c.risk === "safe" ? Color.Green : Color.Orange }}
              accessories={[{ text: human(c.size_bytes) }]}
              actions={
                <ActionPanel>
                  <Action
                    title={c.risk === "safe" ? "Clean This Category" : "Clean (Moves Personal Files to Trash)"}
                    icon={Icon.Trash}
                    onAction={async () => {
                      const toast = await showToast({ style: Toast.Style.Animated, title: `Cleaning ${c.name}…` });
                      try {
                        await run(["clean", "--only", c.id, "--yes"]);
                        toast.style = Toast.Style.Success;
                        toast.title = `Cleaned ${c.name}`;
                      } catch (e) {
                        toast.style = Toast.Style.Failure;
                        toast.title = String((e as Error).message);
                      }
                    }}
                  />
                  <Action.CopyToClipboard title="Copy Command" content={`dustit clean --only ${c.id}`} />
                </ActionPanel>
              }
            />
          ))}
        </List.Section>
      )}
      {groups && (
        <List.Section title="Where the space is">
          {groups
            .filter((g) => g.size_bytes > 1e8)
            .map((g) => (
              <List.Item key={g.name} title={g.name} subtitle={g.hint} accessories={[{ text: human(g.size_bytes) }]} />
            ))}
        </List.Section>
      )}
    </List>
  );
}
