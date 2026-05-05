import type { Metadata } from "next";
import { NimbusLanding } from "@/features/landing/components/nimbus-landing";

export const metadata: Metadata = {
  title: "Homepage",
  description:
    "Nimbus — open-source platform that turns coding agents into real teammates. Assign tasks, track progress, compound skills.",
  openGraph: {
    title: "Nimbus — Project Management for Human + Agent Teams",
    description:
      "Manage your human + agent workforce in one place.",
    url: "/homepage",
  },
  alternates: {
    canonical: "/homepage",
  },
};

export default function HomepagePage() {
  return <NimbusLanding />;
}
