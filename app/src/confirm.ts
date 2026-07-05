import { Alert, Platform } from 'react-native';

interface ConfirmOptions {
  title: string;
  message: string;
  confirmLabel?: string; // default 'OK'
  cancelLabel?: string;  // default 'Cancel'
  destructive?: boolean; // styles the native confirm button; default false
}

/** Resolves true if the user confirms. Works on web (window.confirm) and native (Alert.alert). */
export function confirmAsync(opts: ConfirmOptions): Promise<boolean> {
  const {
    title,
    message,
    confirmLabel = 'OK',
    cancelLabel = 'Cancel',
    destructive = false,
  } = opts;

  if (Platform.OS === 'web') {
    // Alert.alert is a no-op on react-native-web; use the browser's native confirm dialog.
    return Promise.resolve(window.confirm(message));
  }

  return new Promise<boolean>((resolve) => {
    Alert.alert(title, message, [
      { text: cancelLabel, style: 'cancel', onPress: () => resolve(false) },
      {
        text: confirmLabel,
        style: destructive ? 'destructive' : 'default',
        onPress: () => resolve(true),
      },
    ]);
  });
}
