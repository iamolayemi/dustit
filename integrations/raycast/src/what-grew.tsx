import { Color, Icon, List } from "@raycast/api";
import { useEffect, useState } from "react";
import { Change, human, runJSON } from "./dustit";

interface Growth {
  since: string;
  free_before: number;
  free_now: number;
  groups: Change[] | null;
  folders: Change[] | null;
}

const signed = (n: number) => (n >= 0 ? `+${human(n)}` : `−${human(-n)}`);

export default function WhatGrew() {
  const [growth, setGrowth] = useState<Growth>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    runJSON<Growth>(["growth", "--since", "7d"]).then(setGrowth).catch((e) => setError(String(e.message)));
  }, []);
  if (error || (growth && !growth.since)) {
    return (
      <List>
        <List.EmptyView
          icon={Icon.Clock}
          title="No timeline yet"
          description={error ?? "Run `dustit snapshot` (or `dustit schedule watch on`) and check back later."}
        />
      </List>
    );
  }
  const row = (c: Change) => (
    <List.Item
      key={c.name}
      title={c.name}
      icon={{ source: c.delta > 0 ? Icon.ArrowUp : Icon.ArrowDown, tintColor: c.delta > 0 ? Color.Orange : Color.Green }}
      accessories={[{ text: signed(c.delta) }, { text: `${human(c.before)} → ${human(c.after)}` }]}
    />
  );
  return (
    <List isLoading={!growth}>
      {growth && (
        <>
          <List.Section title={`Since ${new Date(growth.since).toLocaleDateString()} · free ${signed(growth.free_now - growth.free_before)}`}>
            {(growth.groups ?? []).map(row)}
          </List.Section>
          <List.Section title="Folders">{(growth.folders ?? []).map(row)}</List.Section>
        </>
      )}
    </List>
  );
}
