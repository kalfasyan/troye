import { isIos } from './platform';

// Installing the app on a phone or desktop. Chrome, Edge and Android offer an
// install prompt the app can trigger; iOS Safari needs Share > Add to Home Screen.

interface InstallPromptEvent extends Event {
  prompt(): Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

class Install {
  installed = $state(matchMedia('(display-mode: standalone)').matches);
  private prompt = $state<InstallPromptEvent | null>(null);

  /** The browser can install the app with one tap. */
  canPrompt = $derived(!this.installed && this.prompt !== null);
  /** The app could be installed by hand from Safari's Share menu. */
  needsIosSteps = $derived(!this.installed && isIos(navigator.userAgent, navigator.maxTouchPoints));

  constructor() {
    addEventListener('beforeinstallprompt', (e) => {
      e.preventDefault();
      this.prompt = e as InstallPromptEvent;
    });
    addEventListener('appinstalled', () => {
      this.installed = true;
      this.prompt = null;
    });
  }

  async install() {
    if (!this.prompt) return;
    await this.prompt.prompt();
    const { outcome } = await this.prompt.userChoice;
    if (outcome === 'accepted') this.installed = true;
    this.prompt = null;
  }
}

export const install = new Install();
