"use client";

import { useActionState, useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { registerAction, type RegisterFormState } from "./actions";

const schema = z.object({
  company_name: z.string().min(2, "Nama perusahaan minimal 2 karakter").max(200),
  package: z.enum(["starter", "growth", "enterprise"]),
  contact_name: z.string().min(1, "Nama kontak wajib diisi").max(200),
  contact_email: z.string().email("Email tidak valid").max(320),
  contact_phone: z.string().max(30).optional().or(z.literal("")),
});

type FormValues = z.infer<typeof schema>;

const initialState: RegisterFormState = { status: "idle" };

function RequiredMark() {
  return (
    <span className="ml-0.5 text-destructive" aria-hidden="true">
      *
    </span>
  );
}

interface RegisterFormProps {
  onSuccess: (email: string) => void;
}

export function RegisterForm({ onSuccess }: RegisterFormProps) {
  const [state, formAction, isPending] = useActionState(registerAction, initialState);

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      company_name: "",
      package: "starter",
      contact_name: "",
      contact_email: "",
      contact_phone: "",
    },
  });

  useEffect(() => {
    if (state.status === "error" && state.toast) {
      toast.error(state.toast);
    }
    if (state.status === "error" && state.fieldErrors) {
      for (const [field, messages] of Object.entries(state.fieldErrors)) {
        if (messages[0]) {
          form.setError(field as keyof FormValues, { message: messages[0] });
        }
      }
    }
    if (state.status === "success") {
      onSuccess(state.contactEmail);
    }
  }, [state, form, onSuccess]);

  return (
    <Form {...form}>
      <form action={formAction} className="space-y-5">
        <FormField
          control={form.control}
          name="company_name"
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                Nama Perusahaan
                <RequiredMark />
              </FormLabel>
              <FormControl>
                <Input placeholder="Acme Spa" autoComplete="organization" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="package"
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                Paket
                <RequiredMark />
              </FormLabel>
              <Select value={field.value} onValueChange={field.onChange} name={field.name}>
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder="Pilih paket" />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value="starter">Starter &mdash; 1 cabang</SelectItem>
                  <SelectItem value="growth">Growth &mdash; 5 cabang</SelectItem>
                  <SelectItem value="enterprise">Enterprise &mdash; cabang tanpa batas</SelectItem>
                </SelectContent>
              </Select>
              <FormDescription>Paket dapat diubah oleh tim kami saat review.</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <div className="grid gap-5 sm:grid-cols-2">
          <FormField
            control={form.control}
            name="contact_name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  Nama Kontak
                  <RequiredMark />
                </FormLabel>
                <FormControl>
                  <Input placeholder="Alice Smith" autoComplete="name" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="contact_phone"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Telepon (opsional)</FormLabel>
                <FormControl>
                  <Input placeholder="+62812xxxxxxx" autoComplete="tel" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        <FormField
          control={form.control}
          name="contact_email"
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                Email Kontak
                <RequiredMark />
              </FormLabel>
              <FormControl>
                <Input
                  type="email"
                  placeholder="admin@acme-spa.example"
                  autoComplete="email"
                  {...field}
                />
              </FormControl>
              <FormDescription>
                Kami akan mengirim kredensial masuk ke email ini setelah disetujui.
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <p className="text-xs text-muted-foreground">
          Kolom bertanda <span className="text-destructive">*</span> wajib diisi.
        </p>

        <Button type="submit" className="w-full" disabled={isPending} aria-busy={isPending}>
          {isPending ? (
            <>
              <Loader2 className="animate-spin" aria-hidden="true" />
              Mengirim permintaan&hellip;
            </>
          ) : (
            "Daftarkan Perusahaan"
          )}
        </Button>
      </form>
    </Form>
  );
}
