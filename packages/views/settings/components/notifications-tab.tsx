"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@nimbus/core/hooks";
import { notificationPreferenceOptions } from "@nimbus/core/notification-preferences/queries";
import { useUpdateNotificationPreferences } from "@nimbus/core/notification-preferences/mutations";
import type { NotificationGroupKey, NotificationPreferences } from "@nimbus/core/types";
import { Card, CardContent } from "@nimbus/ui/components/ui/card";
import { Switch } from "@nimbus/ui/components/ui/switch";
import { toast } from "sonner";

const notificationGroups: {
  key: NotificationGroupKey;
  label: string;
  description: string;
}[] = [
  {
    key: "assignments",
    label: "Assignments",
    description: "When you are assigned or unassigned from an issue",
  },
  {
    key: "status_changes",
    label: "Status changes",
    description: "When an issue you follow changes status (e.g. todo, in progress, done)",
  },
  {
    key: "comments",
    label: "Comments & Mentions",
    description: "New comments on issues you follow, or when someone @mentions you",
  },
  {
    key: "updates",
    label: "Priority & Due date",
    description: "When priority or due date changes on issues you follow",
  },
  {
    key: "agent_activity",
    label: "Agent activity",
    description: "When an agent task completes or fails",
  },
];

export function NotificationsTab() {
  const wsId = useWorkspaceId();
  const { data } = useQuery(notificationPreferenceOptions(wsId));
  const mutation = useUpdateNotificationPreferences();

  const preferences = data?.preferences ?? {};

  const handleToggle = (key: NotificationGroupKey, enabled: boolean) => {
    const updated: NotificationPreferences = {
      ...preferences,
      [key]: enabled ? "all" : "muted",
    };
    // Remove keys set to "all" (default) to keep the object clean
    if (enabled) {
      delete updated[key];
    }
    mutation.mutate(updated, {
      onError: () => toast.error("Failed to update notification settings"),
    });
  };

  return (
    <div className="space-y-4">
      <section className="space-y-4">
        <div>
          <h2 className="text-sm font-semibold">Inbox Notifications</h2>
          <p className="text-sm text-muted-foreground mt-1">
            Control which events generate inbox notifications. Muted event types
            are silently filtered — you can still see them by visiting the issue
            directly.
          </p>
        </div>

        <Card>
          <CardContent className="divide-y">
            {notificationGroups.map((group) => {
              const enabled = preferences[group.key] !== "muted";
              return (
                <div
                  key={group.key}
                  className="flex items-center justify-between py-3 first:pt-0 last:pb-0"
                >
                  <div className="space-y-0.5 pr-4">
                    <p className="text-sm font-medium">{group.label}</p>
                    <p className="text-xs text-muted-foreground">
                      {group.description}
                    </p>
                  </div>
                  <Switch
                    checked={enabled}
                    onCheckedChange={(checked) =>
                      handleToggle(group.key, checked)
                    }
                  />
                </div>
              );
            })}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
