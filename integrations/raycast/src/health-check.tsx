import { Color, Icon, List } from "@raycast/api";
import { useEffect, useState } from "react";
import { Finding, runJSON } from "./dustit";

const tint: Record<Finding["severity"], Color> = { bad: Color.Red, warn: Color.Orange, info: Color.Blue, ok: Color.Green };

export default function HealthCheck() {
  const [findings, setFindings] = useState<Finding[]>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    runJSON<Finding[]>(["doctor"]).then(setFindings).catch((e) => setError(String(e.message)));
  }, []);
  if (error) {
    return (
      <List>
        <List.EmptyView icon={Icon.Warning} title="DustIt couldn't run" description={error} />
      </List>
    );
  }
  return (
    <List isLoading={!findings} isShowingDetail>
      {(findings ?? []).map((f) => (
        <List.Item
          key={f.check}
          title={f.check}
          icon={{ source: Icon.CircleFilled, tintColor: tint[f.severity] }}
          accessories={[{ text: f.short }]}
          detail={<List.Item.Detail markdown={`## ${f.check}\n\n${f.summary}\n\n${(f.advice ?? []).map((a) => `- ${a}`).join("\n")}`} />}
        />
      ))}
    </List>
  );
}
