import { ConnectedPosition, Overlay, OverlayRef } from '@angular/cdk/overlay';
import { ComponentPortal, ComponentType } from '@angular/cdk/portal';
import { Injectable, Injector, inject } from '@angular/core';
import { HINT_POPOVER_MODE } from '@app/components/layout/hints-panel/hints-panel.token';

/**
 * Opens the shared HintsPanelComponent as a compact popover anchored to the clicked
 * "Info" link (replacing the old full-width bottom-sheet). Closes on outside click or
 * Escape. Call from a component's toggleHint handler:
 *
 *   toggleHint(event: MouseEvent, message: string) {
 *       this.hintPopover.open(event.currentTarget as HTMLElement, message);
 *   }
 */
@Injectable({ providedIn: 'root' })
export class HintPopoverService {
    private overlay = inject(Overlay);
    private injector = inject(Injector);
    private activeOverlay: OverlayRef | null = null;
    private openRequest = 0;

    open(origin: HTMLElement, mode: string): void {
        // Dispose any popover already open (e.g. a fast double-click on the "Info" link)
        // so overlays never stack.
        this.activeOverlay?.dispose();
        this.activeOverlay = null;

        // The panel carries every help text in the app, so it is loaded on first use to keep it out of
        // the initial bundle. Only the most recent request opens a popover.
        const request = ++this.openRequest;
        import('@app/components/layout/hints-panel/hints-panel.component')
            .then(({HintsPanelComponent}) => {
                if (request === this.openRequest) {
                    this.attach(origin, mode, HintsPanelComponent);
                }
            })
            .catch((error: unknown) => console.error('Unable to load hints panel', error));
    }

    private attach(origin: HTMLElement, mode: string, component: ComponentType<unknown>): void {
        // Prefer below the link (right edge aligned); fall back to above if there isn't room.
        const below: ConnectedPosition = { originX: 'end', originY: 'bottom', overlayX: 'end', overlayY: 'top', offsetY: 8 };
        const above: ConnectedPosition = { originX: 'end', originY: 'top', overlayX: 'end', overlayY: 'bottom', offsetY: -8 };
        const positionStrategy = this.overlay
            .position()
            .flexibleConnectedTo(origin)
            .withPositions([below, above])
            .withPush(true);

        const overlayRef = this.overlay.create({
            positionStrategy,
            scrollStrategy: this.overlay.scrollStrategies.reposition(),
            hasBackdrop: true,
            backdropClass: 'cdk-overlay-transparent-backdrop',
            panelClass: 'hint-popover',
        });
        this.activeOverlay = overlayRef;

        const portalInjector = Injector.create({
            parent: this.injector,
            providers: [{ provide: HINT_POPOVER_MODE, useValue: mode }],
        });
        overlayRef.attach(new ComponentPortal(component, null, portalInjector));

        const dispose = () => {
            overlayRef.dispose();
            if (this.activeOverlay === overlayRef) {
                this.activeOverlay = null;
            }
        };
        overlayRef.backdropClick().subscribe(dispose);
        overlayRef.keydownEvents().subscribe((e) => {
            if (e.key === 'Escape') {
                dispose();
            }
        });
    }
}
