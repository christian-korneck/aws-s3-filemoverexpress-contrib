import { InjectionToken } from '@angular/core';

/**
 * The hint "mode" (which help section to show), supplied by HintPopoverService via the portal injector.
 * Kept apart from the component so the service can lazy-load the component.
 */
export const HINT_POPOVER_MODE = new InjectionToken<string>('HINT_POPOVER_MODE');
