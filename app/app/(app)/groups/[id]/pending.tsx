import { Redirect, useLocalSearchParams } from 'expo-router';

// This screen is deprecated - pending entries are now shown inline in the ledger
export default function PendingScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  return <Redirect href={`/(app)/groups/${id}/ledger`} />;
}
