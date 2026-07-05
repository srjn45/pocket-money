import { Redirect } from 'expo-router';
import { useAuth } from '../src/auth-context';
import { LoadingSpinner } from '../src/components';

export default function Index() {
  const { isLoading, token } = useAuth();

  if (isLoading) {
    return <LoadingSpinner />;
  }

  if (token) {
    return <Redirect href="/(app)" />;
  }

  return <Redirect href="/(auth)/login" />;
}
