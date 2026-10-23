//go:build darwin

#import <Cocoa/Cocoa.h>

extern void goOnToggleMaster(void);
extern void goOnToggleAccount(int index);
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

@implementation MacTrayHandler

- (void)actionToggleMaster:(id)sender { goOnToggleMaster(); }
- (void)actionToggleAccount:(id)sender { goOnToggleAccount((int)[(NSMenuItem *)sender tag]); }
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

    // 1. Cabecera informativa con semáforo
    BOOL effectivelyActive = NO;
    if (masterActive) {
        for (NSDictionary *acc in accounts) {
            if ([acc[@"enabled"] boolValue]) { effectivelyActive = YES; break; }
        }
    }

    NSString *headerTitle = @"🔴 CloudflareNetConnect (Servicio Desactivado)";
    if (effectivelyActive) {
        headerTitle = @"🟢 CloudflareNetConnect (Servicio Activo)";
    } else if (masterActive && accounts.count == 0) {
        headerTitle = @"🔴 CloudflareNetConnect (Sin dominios vinculados)";
    } else if (masterActive) {
        headerTitle = @"🔴 CloudflareNetConnect (Sin dominios activos)";
    }
    NSMenuItem *header = [[NSMenuItem alloc] initWithTitle:headerTitle action:nil keyEquivalent:@""];
    [header setEnabled:NO];
    [menu addItem:header];
    [menu addItem:[NSMenuItem separatorItem]];

    // 2. Interruptor Maestro
    NSString *masterTitle = masterActive ? @"⚡ Servicio de Túneles Activo (Clic para Desactivar)" : @"⚡ Activar Servicio de Túneles (Actualmente OFF)";
    NSMenuItem *masterItem = [[NSMenuItem alloc] initWithTitle:masterTitle action:@selector(actionToggleMaster:) keyEquivalent:@""];
    [masterItem setTarget:self];
    [masterItem setState:masterActive ? NSControlStateValueOn : NSControlStateValueOff];
    [menu addItem:masterItem];
    [menu addItem:[NSMenuItem separatorItem]];

    // 3. Sección de Cuentas con Submenú idéntico a Windows
    for (int i = 0; i < (int)accounts.count; i++) {
        NSDictionary *acc = accounts[i];
        NSString *name = acc[@"name"] ?: @"Cuenta Cloudflare";
        BOOL enabled = [acc[@"enabled"] boolValue];
        NSArray *routes = acc[@"routes"] ?: @[];
        BOOL isOnline = enabled && masterActive;

        NSMenu *subMenu = [[NSMenu alloc] initWithTitle:name];
        [subMenu setAutoenablesItems:NO];

        NSString *stateLabel = enabled ? @"Dominio Activo (Clic para Desactivar)" : @"Activar conexión de este dominio";
        NSMenuItem *toggleItem = [[NSMenuItem alloc] initWithTitle:stateLabel action:@selector(actionToggleAccount:) keyEquivalent:@""];
        [toggleItem setTarget:self];
        [toggleItem setTag:i];
        [toggleItem setState:enabled ? NSControlStateValueOn : NSControlStateValueOff];
        [subMenu addItem:toggleItem];

        NSMenuItem *refreshItem = [[NSMenuItem alloc] initWithTitle:@"🔄 Actualizar túneles" action:@selector(actionRefreshAccount:) keyEquivalent:@""];
        [refreshItem setTarget:self];
        [refreshItem setTag:i];
        [subMenu addItem:refreshItem];

        if (routes.count > 0) {
            [subMenu addItem:[NSMenuItem separatorItem]];
            for (NSString *rLabel in routes) {
                NSMenuItem *rItem = [[NSMenuItem alloc] initWithTitle:rLabel action:nil keyEquivalent:@""];
                [rItem setEnabled:NO];
                [subMenu addItem:rItem];
            }
        }

        [subMenu addItem:[NSMenuItem separatorItem]];
        NSMenuItem *removeItem = [[NSMenuItem alloc] initWithTitle:@"🗑️ Desvincular cuenta" action:@selector(actionRemoveAccount:) keyEquivalent:@""];
        [removeItem setTarget:self];
        [removeItem setTag:i];
        [subMenu addItem:removeItem];

        NSString *accTitle = [NSString stringWithFormat:@"%@ %@", isOnline ? @"🟢" : @"🔴", name];
        NSMenuItem *accParent = [[NSMenuItem alloc] initWithTitle:accTitle action:nil keyEquivalent:@""];
        [accParent setSubmenu:subMenu];
        [menu addItem:accParent];
    }

    if (accounts.count > 0) {
        [menu addItem:[NSMenuItem separatorItem]];
    }

    // 4. Asociar nueva cuenta
    NSMenuItem *linkItem = [[NSMenuItem alloc] initWithTitle:@"➕ Asociar nueva cuenta..." action:@selector(actionLinkAccount:) keyEquivalent:@""];
    [linkItem setTarget:self];
    [menu addItem:linkItem];

    // 5. Submenú Servicio del Sistema (Autoarranque) idéntico a Windows
    NSMenu *svcMenu = [[NSMenu alloc] initWithTitle:@"Servicio del Sistema"];
    [svcMenu setAutoenablesItems:NO];
    BOOL isSvcInstalled = (svcState != 0);

    NSString *autoText = isSvcInstalled ? @"Iniciar al encender el Mac (Gestionado por Servicio)" : @"Iniciar aplicación al encender el Mac";
    NSMenuItem *autoItem = [[NSMenuItem alloc] initWithTitle:autoText action:@selector(actionToggleStartup:) keyEquivalent:@""];
    [autoItem setTarget:self];
    [autoItem setState:(isSvcInstalled || startupEnabled) ? NSControlStateValueOn : NSControlStateValueOff];
    if (isSvcInstalled) [autoItem setEnabled:NO];
    [svcMenu addItem:autoItem];
    [svcMenu addItem:[NSMenuItem separatorItem]];

    if (svcState == 4) {
        NSMenuItem *st = [[NSMenuItem alloc] initWithTitle:@"🟢 Servicio macOS: Activo (2º Plano)" action:nil keyEquivalent:@""];
        [st setEnabled:NO];
        [svcMenu addItem:st];
        [svcMenu addItem:[NSMenuItem separatorItem]];
        NSMenuItem *bStop = [[NSMenuItem alloc] initWithTitle:@"⏹️ Detener Servicio macOS" action:@selector(actionSvcStop:) keyEquivalent:@""];
        [bStop setTarget:self];
        [svcMenu addItem:bStop];
        NSMenuItem *bUn = [[NSMenuItem alloc] initWithTitle:@"🗑️ Desinstalar Servicio macOS" action:@selector(actionSvcUninstall:) keyEquivalent:@""];
        [bUn setTarget:self];
        [svcMenu addItem:bUn];
    } else if (svcState == 1) {
        NSMenuItem *st = [[NSMenuItem alloc] initWithTitle:@"🔴 Servicio macOS: Detenido (Instalado)" action:nil keyEquivalent:@""];
        [st setEnabled:NO];
        [svcMenu addItem:st];
        [svcMenu addItem:[NSMenuItem separatorItem]];
        NSMenuItem *bStart = [[NSMenuItem alloc] initWithTitle:@"▶️ Iniciar Servicio macOS" action:@selector(actionSvcStart:) keyEquivalent:@""];
        [bStart setTarget:self];
        [svcMenu addItem:bStart];
        NSMenuItem *bUn = [[NSMenuItem alloc] initWithTitle:@"🗑️ Desinstalar Servicio macOS" action:@selector(actionSvcUninstall:) keyEquivalent:@""];
        [bUn setTarget:self];
        [svcMenu addItem:bUn];
    } else {
        NSMenuItem *st = [[NSMenuItem alloc] initWithTitle:@"🔴 Servicio macOS: No Instalado" action:nil keyEquivalent:@""];
        [st setEnabled:NO];
        [svcMenu addItem:st];
        [svcMenu addItem:[NSMenuItem separatorItem]];
        NSMenuItem *bInst = [[NSMenuItem alloc] initWithTitle:@"➕ Instalar y Activar Servicio macOS (2º Plano)" action:@selector(actionSvcInstall:) keyEquivalent:@""];
        [bInst setTarget:self];
        [svcMenu addItem:bInst];
    }

    NSString *svcDot = (svcState == 4 || startupEnabled) ? @"🟢" : @"🔴";
    NSString *svcParentTitle = [NSString stringWithFormat:@"%@ ⚙️ Servicio del Sistema (Autoarranque)", svcDot];
    NSMenuItem *svcParent = [[NSMenuItem alloc] initWithTitle:svcParentTitle action:nil keyEquivalent:@""];
    [svcParent setSubmenu:svcMenu];
    [menu addItem:svcParent];

    // 6. Abrir carpeta de configuración y Salir
    NSMenuItem *cfgItem = [[NSMenuItem alloc] initWithTitle:@"📂 Abrir Carpeta de Configuración..." action:@selector(actionOpenConfig:) keyEquivalent:@""];
    [cfgItem setTarget:self];
    [menu addItem:cfgItem];
    [menu addItem:[NSMenuItem separatorItem]];

    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"❌ Salir" action:@selector(actionQuit:) keyEquivalent:@"q"];
    [quitItem setTarget:self];
    [menu addItem:quitItem];

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
        NSDictionary *state = @{};
        if (jsonData) {
            state = [NSJSONSerialization JSONObjectWithData:jsonData options:0 error:nil] ?: @{};
        }
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
