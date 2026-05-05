"use client";

import { DashboardLayout } from "@nimbus/views/layout";
import { NimbusIcon as NimbusIcon } from "@nimbus/ui/components/common/nimbus-icon";
import { SearchCommand, SearchTrigger } from "@nimbus/views/search";
import { ChatFab, ChatWindow } from "@nimbus/views/chat";
import { StarterContentPrompt } from "@nimbus/views/onboarding";

export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <DashboardLayout
      loadingIndicator={<NimbusIcon className="size-6" />}
      searchSlot={<SearchTrigger />}
      extra={
        <>
          <SearchCommand />
          <ChatWindow />
          <ChatFab />
          <StarterContentPrompt />
        </>
      }
    >
      {children}
    </DashboardLayout>
  );
}
