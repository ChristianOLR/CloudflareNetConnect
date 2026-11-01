//go:build darwin

#import <Cocoa/Cocoa.h>

extern void goOnToggleMaster(void);
extern void goOnToggleAccount(int index);
extern void goOnTogglePn(int index);
extern void goOnTogglePnAuto(int index);
extern void goOnRefreshAccount(int index);
extern void goOnRemoveAccount(int index);
extern void goOnLinkAccount(void);
extern void goOnToggleStartup(void);
extern void goOnServiceAction(const char* action);
extern void goOnOpenConfig(void);
extern void goOnQuit(void);

@interface MacTrayHandler : NSObject
@property (strong, nonatomic) NSStatusItem *statusItem;
@end

static MacTrayHandler *g_trayHandler = nil;

static NSMenuItem* makeItem(NSString *title, SEL action, id target, NSInteger tag, NSControlStateValue state) {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:@""];
    [item setTarget:target];
    [item setTag:tag];
    [item setState:state];
    return item;
}

@implementation MacTrayHandler

- (void)actionToggleMaster:(id)sender { goOnToggleMaster(); }
- (void)actionToggleAccount:(id)sender { goOnToggleAccount((int)[(NSMenuItem *)sender tag]); }
- (void)actionTogglePn:(id)sender { goOnTogglePn((int)[(NSMenuItem *)sender tag]); }
- (void)actionTogglePnAuto:(id)sender { goOnTogglePnAuto((int)[(NSMenuItem *)sender tag]); }
- (void)actionRefreshAccount:(id)sender { goOnRefreshAccount((int)[(NSMenuItem *)sender tag]); }
- (void)actionRemoveAccount:(id)sender { goOnRemoveAccount((int)[(NSMenuItem *)sender tag]); }
- (void)actionLinkAccount:(id)sender { goOnLinkAccount(); }
- (void)actionToggleStartup:(id)sender { goOnToggleStartup(); }
- (void)actionSvcInstall:(id)sender { goOnServiceAction("install_start"); }
- (void)actionSvcStart:(id)sender { goOnServiceAction("start"); }
- (void)actionSvcStop:(id)sender { goOnServiceAction("stop"); }
- (void)actionSvcUninstall:(id)sender { goOnServiceAction("stop_uninstall"); }
- (void)actionOpenConfig:(id)sender { goOnOpenConfig(); }
- (void)actionQuit:(id)sender { goOnQuit(); }

- (void)setupMenuWithState:(NSDictionary *)state {
    BOOL masterActive = [state[@"master_active"] boolValue];
    BOOL startupEnabled = [state[@"startup_enabled"] boolValue];
    int svcState = [state[@"svc_state"] intValue];
    NSArray *accounts = state[@"accounts"] ?: @[];

    NSMenu *menu = [[NSMenu alloc] initWithTitle:@"CloudflareNetConnect"];
    [menu setAutoenablesItems:NO];

    BOOL effActive = NO;
    if (masterActive) {
        for (NSDictionary *acc in accounts) {
            if ([acc[@"enabled"] boolValue]) { effActive = YES; break; }
        }
    }

    NSString *headerTitle = @"🔴 CloudflareNetConnect (Servicio Desactivado)";
    if (effActive) {
        headerTitle = @"🟢 CloudflareNetConnect (Servicio Activo)";
    } else if (masterActive) {
        headerTitle = (accounts.count == 0) ? @"🔴 CloudflareNetConnect (Sin dominios vinculados)" : @"🔴 CloudflareNetConnect (Sin dominios activos)";
    }
    NSMenuItem *header = makeItem(headerTitle, nil, nil, 0, NSControlStateValueOff);
    [header setEnabled:NO];
    [menu addItem:header];
    [menu addItem:[NSMenuItem separatorItem]];

    NSString *mTitle = masterActive ? @"⚡ Servicio de Túneles Activo (Clic para Desactivar)" : @"⚡ Activar Servicio de Túneles (Actualmente OFF)";
    [menu addItem:makeItem(mTitle, @selector(actionToggleMaster:), self, 0, masterActive ? NSControlStateValueOn : NSControlStateValueOff)];
    [menu addItem:[NSMenuItem separatorItem]];

    for (int i = 0; i < (int)accounts.count; i++) {
        NSDictionary *acc = accounts[i];
        NSString *name = acc[@"name"] ?: @"Cuenta Cloudflare";
        BOOL enabled = [acc[@"enabled"] boolValue];
        BOOL isOnline = enabled && masterActive;
        NSArray *routes = acc[@"routes"] ?: @[];

        NSMenu *sub = [[NSMenu alloc] initWithTitle:name];
        [sub setAutoenablesItems:NO];

        NSString *sLabel = enabled ? @"Dominio Activo (Clic para Desactivar)" : @"Activar conexión de este dominio";
        [sub addItem:makeItem(sLabel, @selector(actionToggleAccount:), self, i, enabled ? NSControlStateValueOn : NSControlStateValueOff)];
        [sub addItem:makeItem(@"🔄 Actualizar túneles", @selector(actionRefreshAccount:), self, i, NSControlStateValueOff)];

        if ([acc[@"pn_has_config"] boolValue]) {
            NSString *pnLabel = acc[@"pn_label"] ?: @"🌐 Red Privada";
            BOOL pnEnabled = [acc[@"pn_enabled"] boolValue];
            [sub addItem:makeItem(pnLabel, @selector(actionTogglePn:), self, i, pnEnabled ? NSControlStateValueOn : NSControlStateValueOff)];

            if ([acc[@"pn_show_auto"] boolValue]) {
                BOOL autoDet = [acc[@"pn_auto_detect"] boolValue];
                [sub addItem:makeItem(@"   ↪ 🔄 Auto-Bypass en Red Local", @selector(actionTogglePnAuto:), self, i, autoDet ? NSControlStateValueOn : NSControlStateValueOff)];
            }
        }

        if (routes.count > 0) {
            [sub addItem:[NSMenuItem separatorItem]];
            for (NSString *r in routes) {
                NSMenuItem *rItem = makeItem(r, nil, nil, 0, NSControlStateValueOff);
                [rItem setEnabled:NO];
                [sub addItem:rItem];
            }
        }
        [sub addItem:[NSMenuItem separatorItem]];
        [sub addItem:makeItem(@"🗑️ Desvincular cuenta", @selector(actionRemoveAccount:), self, i, NSControlStateValueOff)];

        NSMenuItem *parent = makeItem([NSString stringWithFormat:@"%@ %@", isOnline ? @"🟢" : @"🔴", name], nil, nil, 0, NSControlStateValueOff);
        [parent setSubmenu:sub];
        [menu addItem:parent];
    }
    if (accounts.count > 0) [menu addItem:[NSMenuItem separatorItem]];

    [menu addItem:makeItem(@"➕ Asociar nueva cuenta...", @selector(actionLinkAccount:), self, 0, NSControlStateValueOff)];

    NSMenu *svcMenu = [[NSMenu alloc] initWithTitle:@"Servicio del Sistema"];
    [svcMenu setAutoenablesItems:NO];
    BOOL isSvcInstalled = (svcState != 0);
    NSString *autoText = isSvcInstalled ? @"Iniciar al encender el Mac (Gestionado por Servicio)" : @"Iniciar aplicación al encender el Mac";
    NSMenuItem *autoItem = makeItem(autoText, @selector(actionToggleStartup:), self, 0, (isSvcInstalled || startupEnabled) ? NSControlStateValueOn : NSControlStateValueOff);
    if (isSvcInstalled) [autoItem setEnabled:NO];
    [svcMenu addItem:autoItem];
    [svcMenu addItem:[NSMenuItem separatorItem]];

    if (svcState == 4) {
        NSMenuItem *st = makeItem(@"🟢 Servicio macOS: Activo (2º Plano)", nil, nil, 0, NSControlStateValueOff);
        [st setEnabled:NO];
        [svcMenu addItem:st];
        [svcMenu addItem:[NSMenuItem separatorItem]];
        [svcMenu addItem:makeItem(@"⏹️ Detener Servicio macOS", @selector(actionSvcStop:), self, 0, NSControlStateValueOff)];
        [svcMenu addItem:makeItem(@"🗑️ Desinstalar Servicio macOS", @selector(actionSvcUninstall:), self, 0, NSControlStateValueOff)];
    } else if (svcState == 1) {
        NSMenuItem *st = makeItem(@"🔴 Servicio macOS: Detenido (Instalado)", nil, nil, 0, NSControlStateValueOff);
        [st setEnabled:NO];
        [svcMenu addItem:st];
        [svcMenu addItem:[NSMenuItem separatorItem]];
        [svcMenu addItem:makeItem(@"▶️ Iniciar Servicio macOS", @selector(actionSvcStart:), self, 0, NSControlStateValueOff)];
        [svcMenu addItem:makeItem(@"🗑️ Desinstalar Servicio macOS", @selector(actionSvcUninstall:), self, 0, NSControlStateValueOff)];
    } else {
        NSMenuItem *st = makeItem(@"🔴 Servicio macOS: No Instalado", nil, nil, 0, NSControlStateValueOff);
        [st setEnabled:NO];
        [svcMenu addItem:st];
        [svcMenu addItem:[NSMenuItem separatorItem]];
        [svcMenu addItem:makeItem(@"➕ Instalar y Activar Servicio macOS (2º Plano)", @selector(actionSvcInstall:), self, 0, NSControlStateValueOff)];
    }

    NSString *svcDot = (svcState == 4 || startupEnabled) ? @"🟢" : @"🔴";
    NSMenuItem *svcParent = makeItem([NSString stringWithFormat:@"%@ ⚙️ Servicio del Sistema (Autoarranque)", svcDot], nil, nil, 0, NSControlStateValueOff);
    [svcParent setSubmenu:svcMenu];
    [menu addItem:svcParent];

    [menu addItem:makeItem(@"📂 Abrir Carpeta de Configuración...", @selector(actionOpenConfig:), self, 0, NSControlStateValueOff)];
    [menu addItem:[NSMenuItem separatorItem]];
    [menu addItem:makeItem(@"❌ Salir", @selector(actionQuit:), self, 0, NSControlStateValueOff)];

    self.statusItem.menu = menu;
}

@end

void initMacTray(const void* iconBytes, int iconLen) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        g_trayHandler = [[MacTrayHandler alloc] init];
        g_trayHandler.statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
        if (iconBytes != NULL && iconLen > 0) {
            NSImage *img = [[NSImage alloc] initWithData:[NSData dataWithBytes:iconBytes length:iconLen]];
            if (img) {
                [img setSize:NSMakeSize(18, 18)];
                [img setTemplate:NO];
                g_trayHandler.statusItem.button.image = img;
            }
        }
    }
}

void setMacMenu(const char* stateJson, const void* iconBytes, int iconLen) {
    @autoreleasepool {
        NSString *jsonStr = [NSString stringWithUTF8String:stateJson];
        NSData *jsonData = [jsonStr dataUsingEncoding:NSUTF8StringEncoding];
        NSDictionary *state = jsonData ? [NSJSONSerialization JSONObjectWithData:jsonData options:0 error:nil] : @{};
        if (!state) state = @{};
        NSData *iconData = (iconBytes != NULL && iconLen > 0) ? [NSData dataWithBytes:iconBytes length:iconLen] : nil;

        dispatch_async(dispatch_get_main_queue(), ^{
            if (iconData) {
                NSImage *img = [[NSImage alloc] initWithData:iconData];
                if (img) {
                    [img setSize:NSMakeSize(18, 18)];
                    [img setTemplate:NO];
                    g_trayHandler.statusItem.button.image = img;
                    g_trayHandler.statusItem.button.title = @"";
                }
            }
            [g_trayHandler setupMenuWithState:state];
        });
    }
}

void runMacLoop(void) { @autoreleasepool { [NSApp run]; } }

void stopMacLoop(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp stop:nil];
        NSEvent *ev = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSMakePoint(0, 0) modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
        [NSApp postEvent:ev atStart:YES];
    });
}
