import { Redirect, useLocalSearchParams } from 'expo-router';

// This screen is deprecated - settlements are now handled via the ledger
export default function SettlementsScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  return <Redirect href={`/(app)/groups/${id}/ledger`} />;
}
