import { redirect } from "next/navigation";

export default function LegacyLiveKitPage() {
  redirect("/console?tab=test");
}
